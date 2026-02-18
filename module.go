package doormonitor

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"go.viam.com/rdk/components/board"
	"go.viam.com/rdk/components/sensor"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	// DoorMonitor is the model for the doormonitor module.
	DoorMonitor      = resource.NewModel("clint", "door-monitor", "door-monitor")
	errUnimplemented = errors.New("unimplemented")
)

func init() {
	resource.RegisterComponent(sensor.API, DoorMonitor,
		resource.Registration[sensor.Sensor, *Config]{
			Constructor: newDoorMonitorDoorMonitor,
		},
	)
}

// Config configuration for the door monitor module.
type Config struct {
	BoardName      string `json:"board_name"`
	SensorPin      string `json:"sensor_pin"`
	SensorType     string `json:"sensor_type"` // "NO" or "NC", default "NO"
	GreenLightPin  string `json:"green_light_pin"`
	YellowLightPin string `json:"yellow_light_pin"`
	RedLightPin    string `json:"red_light_pin"`
	WarningTime    int    `json:"warning_time"` // default 60
}

// Validate ensures all parts of the config are valid and important fields exist.
func (cfg *Config) Validate(path string) ([]string, []string, error) {
	var deps []string
	if cfg.BoardName == "" {
		return nil, nil, fmt.Errorf("board_name is required")
	}
	deps = append(deps, cfg.BoardName)

	if cfg.SensorPin == "" {
		return nil, nil, fmt.Errorf("sensor_pin is required")
	}

	if cfg.WarningTime == 0 {
		cfg.WarningTime = 60
	}
	if cfg.SensorType == "" {
		cfg.SensorType = "NO"
	}
	if cfg.SensorType != "NO" && cfg.SensorType != "NC" {
		return nil, nil, fmt.Errorf("sensor_type must be 'NO' or 'NC'")
	}

	return deps, nil, nil
}

type doorMonitorDoorMonitor struct {
	resource.AlwaysRebuild

	name resource.Name

	logger logging.Logger
	cfg    *Config

	cancelCtx  context.Context
	cancelFunc func()

	board board.Board

	sensorPin   board.GPIOPin
	greenLight  board.GPIOPin
	yellowLight board.GPIOPin
	redLight    board.GPIOPin

	mu               sync.Mutex
	doorState        string    // "open" or "closed"
	openTime         time.Time // When the door opened
	lastWarning      time.Time
	closedReported   bool    // Whether we've reported the closed state to data manager
	lastOpenDuration float64 // Duration the door was open (set on close)
}

func newDoorMonitorDoorMonitor(ctx context.Context, deps resource.Dependencies, rawConf resource.Config, logger logging.Logger) (sensor.Sensor, error) {
	conf, err := resource.NativeConfig[*Config](rawConf)
	if err != nil {
		return nil, err
	}

	return NewDoorMonitor(ctx, deps, rawConf.ResourceName(), conf, logger)

}

func NewDoorMonitor(ctx context.Context, deps resource.Dependencies, name resource.Name, conf *Config, logger logging.Logger) (sensor.Sensor, error) {
	b, err := board.FromDependencies(deps, conf.BoardName)
	if err != nil {
		return nil, fmt.Errorf("failed to get board %q: %w", conf.BoardName, err)
	}

	cancelCtx, cancelFunc := context.WithCancel(context.Background())

	s := &doorMonitorDoorMonitor{
		name:       name,
		logger:     logger,
		cfg:        conf,
		cancelCtx:  cancelCtx,
		cancelFunc: cancelFunc,
		board:      b,
		doorState:  "closed",
	}

	if err := s.configurePins(ctx); err != nil {
		// Log error but maybe don't fail startup if transient?
		// Better to fail so user knows config is wrong.
		cancelFunc()
		return nil, err
	}

	// Start background polling
	s.startPolling()

	return s, nil
}

func (s *doorMonitorDoorMonitor) configurePins(ctx context.Context) error {
	// Sensor Pin
	pin, err := s.board.GPIOPinByName(s.cfg.SensorPin)
	if err != nil {
		return fmt.Errorf("sensor pin %s not found: %w", s.cfg.SensorPin, err)
	}
	s.sensorPin = pin
	// Usually reed switches might need pull-up if not hardware provided.
	// We'll assume simple input for now or let Board config handle electrical properties if possible.
	// But commonly we might want to set it to input.
	// s.sensorPin.Set(ctx, false) // Not exactly 'Set', we read from it.

	// Light Pins
	if s.cfg.GreenLightPin != "" {
		p, err := s.board.GPIOPinByName(s.cfg.GreenLightPin)
		if err != nil {
			return fmt.Errorf("green light pin %s not found: %w", s.cfg.GreenLightPin, err)
		}
		s.greenLight = p
	}
	if s.cfg.YellowLightPin != "" {
		p, err := s.board.GPIOPinByName(s.cfg.YellowLightPin)
		if err != nil {
			return fmt.Errorf("yellow light pin %s not found: %w", s.cfg.YellowLightPin, err)
		}
		s.yellowLight = p
	}
	if s.cfg.RedLightPin != "" {
		p, err := s.board.GPIOPinByName(s.cfg.RedLightPin)
		if err != nil {
			return fmt.Errorf("red light pin %s not found: %w", s.cfg.RedLightPin, err)
		}
		s.redLight = p
	}

	return nil
}

func (s *doorMonitorDoorMonitor) startPolling() {
	go func() {
		ticker := time.NewTicker(250 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-s.cancelCtx.Done():
				return
			case <-ticker.C:
				s.monitorLoop()
			}
		}
	}()
}

func (s *doorMonitorDoorMonitor) monitorLoop() {
	// 1. Read Sensor
	// "High" usually means Open for NO switches, but depends on wiring.
	// User said: "if its NC or NO (default to NO)".
	// NO (Normally Open): Circuit open when magnet away (Door Open) -> Pin High (if pulled up) or Low (if pulled down).
	// Let's assume standard: Door Closed (Magnet near) -> Switch Closed. Door Open (Magnet away) -> Switch Open.
	// If Pin has Pull-Up: Switch Open -> High. Switch Closed -> Low (Ground).
	// So for NO switch: Door Open -> High. Door Closed -> Low.
	// If User selected NC: Door Open -> Closed -> Low. Door Closed -> Open -> High.
	// We'll trust the pin reading.
	// NOTE: Viam Board API `Get` returns bool.
	// We need to know if High is "Active" or Low is "Active".
	// Typically:
	// NO Switch + Pull-Up: Open=High(True), Closed=Low(False).
	// NC Switch + Pull-Up: Open=Low(False), Closed=High(True).

	isHigh, err := s.sensorPin.Get(context.Background(), nil)
	if err != nil {
		s.logger.Errorw("failed to read sensor pin", "error", err)
		return
	}

	// Determine if "Door is Open" based on config
	isOpen := false
	if s.cfg.SensorType == "NO" {
		// NO: Open circuit when door open. With Internal Pull-up (common), Open=High=True.
		// If user has Pull-Down equivalent... checking high usually works for "active" in many board impls or raw GPIO.
		isOpen = isHigh
	} else {
		// NC: Closed circuit when door open. Closed=Low=False?
		// Wait, NO means "Normally Open" (when not actuated/magnet away).
		// Magnet present (Door Closed) -> Actuated -> Closed.
		// Magnet away (Door Open) -> Released -> Open.

		// So for NO switch:
		// Door Closed (Magnet) -> Switch Closed.
		// Door Open (No Magnet) -> Switch Open.

		// If Pull-Up: Switch Closed -> Ground (Low). Switch Open -> VCC (High).
		// So NO + PullUp -> Door Open is High (True). Door Closed is Low (False).

		// If NC switch:
		// Door Closed (Magnet) -> Switch Open.
		// Door Open (No Magnet) -> Switch Closed.
		// If Pull-Up: Door Closed -> Open -> High (True). Door Open -> Closed -> Low (False).

		// So:
		// NO: Open = High
		// NC: Open = Low

		// Simplified logic:
		if s.cfg.SensorType == "NC" {
			isOpen = !isHigh
		} else {
			isOpen = isHigh
		}
	}

	s.mu.Lock()
	previousState := s.doorState
	s.mu.Unlock()

	// State Update

	// We only care about Open vs Closed transitions and duration
	if isOpen {
		if previousState == "closed" {
			// Transition Closed -> Open
			s.mu.Lock()
			s.doorState = "open"
			s.openTime = time.Now()
			s.lastWarning = time.Time{} // Reset warning
			s.closedReported = false
			s.mu.Unlock()

			s.logger.Info("Door Opened")

		} else {
			// Still Open
			// Check Warning
			s.mu.Lock()
			duration := time.Since(s.openTime)
			s.mu.Unlock()

			warningThreshold := time.Duration(s.cfg.WarningTime) * time.Second
			if duration > warningThreshold {
				// Warning State
				s.setLights(false, false, true) // Red
				// Maybe post warning data or log?
			} else {
				// Normal Open
				s.setLights(false, true, false) // Yellow
			}

			// "update it" -> maybe post periodically?
			// User said: "internally i imagine we'd be polling every .5 seconds... update it."
			// But also "post tabular data... for how long."
			// Posting every 0.5s is too much for Data Service probably.
			// Let's post every 10 seconds or so if open?
			// Or just on Close? User said "post tabular data once the door closes. But only once. and then not post more data."
			// But also "as soon as the door opens i'd like to use data manger to post... and then also for how long."
			// I'll stick to Open and Close events for now to save bandwidth, maybe update if it's been open a long time (warning).
		}
	} else {
		// Door is Closed
		if previousState == "open" {
			// Transition Open -> Closed
			s.mu.Lock()
			duration := time.Since(s.openTime).Seconds()
			s.doorState = "closed"
			s.lastOpenDuration = duration
			s.closedReported = false
			s.mu.Unlock()

			s.logger.Info("Door Closed", "duration", duration)
			s.setLights(true, false, false) // Green
		} else {
			// Still Closed
			// Ensure Green is on (idempotent-ish)
			s.setLights(true, false, false)
		}
	}
}

func (s *doorMonitorDoorMonitor) setLights(green, yellow, red bool) {
	if s.greenLight != nil {
		if err := s.greenLight.Set(context.Background(), green, nil); err != nil {
			s.logger.Errorw("failed to set green light", "error", err)
		}
	}
	if s.yellowLight != nil {
		if err := s.yellowLight.Set(context.Background(), yellow, nil); err != nil {
			s.logger.Errorw("failed to set yellow light", "error", err)
		}
	}
	if s.redLight != nil {
		if err := s.redLight.Set(context.Background(), red, nil); err != nil {
			s.logger.Errorw("failed to set red light", "error", err)
		}
	}
}

// Readings implements sensor.Sensor. Data manager calls this to capture door state.
// When the door is closed and we've already reported it once, we return
// ErrNoCaptureToStore (gRPC FailedPrecondition) to signal there's no new data worth storing.
func (s *doorMonitorDoorMonitor) Readings(ctx context.Context, extra map[string]interface{}) (map[string]interface{}, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	fromDM, _ := extra["fromDataManagement"].(bool)
	if fromDM && s.doorState == "closed" && s.closedReported {
		return nil, status.Error(codes.FailedPrecondition, "no capture to store")
	}

	duration := 0.0
	if s.doorState == "open" {
		duration = time.Since(s.openTime).Seconds()
	} else {
		// Door just closed — report the final open duration once
		duration = s.lastOpenDuration
		s.closedReported = true
	}

	return map[string]interface{}{
		"state":      s.doorState,
		"open_time":  duration,
		"is_warning": s.checkWarning(duration),
	}, nil
}

func (s *doorMonitorDoorMonitor) checkWarning(duration float64) bool {
	if duration <= 0 {
		return false
	}
	return duration > float64(s.cfg.WarningTime)
}

func (s *doorMonitorDoorMonitor) Name() resource.Name {

	return s.name
}

func (s *doorMonitorDoorMonitor) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return nil, errUnimplemented
}

func (s *doorMonitorDoorMonitor) Close(context.Context) error {
	// Put close code here
	s.cancelFunc()
	return nil
}
