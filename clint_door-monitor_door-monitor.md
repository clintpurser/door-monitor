# Model clint:door-monitor:door-monitor

The **Door Monitor** is a sensor component that tracks the state of a door (Open/Closed) using a reed switch connected to a Board component's GPIO pin. It provides visual feedback via status lights (Green/Yellow/Red) and exposes door state through the standard sensor `Readings` API for use with the Viam Data Manager.

## Features

- **Door State Monitoring**: Detects if door is open or closed via GPIO.
- **Visual Feedback**:
  - **Green**: Door Closed.
  - **Yellow**: Door Open (normal duration).
  - **Red**: Door Open too long (Warning).
- **Sensor Readings**: Implements the standard `sensor.Readings` API, returning door state, open duration, and warning status.
- **Smart Data Capture**: When the door is closed, returns `ErrNoCaptureToStore` (gRPC `FailedPrecondition`) after the first reading, so the Data Manager only stores data when something interesting is happening.

## Configuration

Add the `door-monitor` sensor to your machine configuration.

### Attributes

| Name               | Type   | Inclusion    | Description                                                                        |
| ------------------ | ------ | ------------ | ---------------------------------------------------------------------------------- |
| `board_name`       | string | **Required** | Name of the Board component managing the GPIO pins.                                |
| `sensor_pin`       | string | **Required** | GPIO pin name/number for the reed switch.                                          |
| `sensor_type`      | string | Optional     | Switch type: `"NO"` (Normally Open, default) or `"NC"` (Normally Closed).          |
| `green_light_pin`  | string | Optional     | GPIO pin for the "Closed" status light.                                            |
| `yellow_light_pin` | string | Optional     | GPIO pin for the "Open" status light.                                              |
| `red_light_pin`    | string | Optional     | GPIO pin for the "Warning" status light.                                           |
| `warning_time`     | int    | Optional     | Duration in seconds before triggering the Warning state (Red light). Default: 60s. |

### Example Configuration

```json
{
  "name": "my-door-monitor",
  "model": "clint:door-monitor:door-monitor",
  "type": "sensor",
  "namespace": "rdk",
  "attributes": {
    "board_name": "local-board",
    "sensor_pin": "8",
    "sensor_type": "NO",
    "green_light_pin": "10",
    "yellow_light_pin": "12",
    "red_light_pin": "16",
    "warning_time": 60
  },
  "depends_on": ["local-board"]
}
```

## Readings

The `Readings` method returns the current door state. Configure the **Data Manager** service to capture from this sensor at your desired interval.

### Response

```json
{
  "state": "open",
  "open_time": 15.5,
  "is_warning": false
}
```

| Field        | Type   | Description                                               |
| ------------ | ------ | --------------------------------------------------------- |
| `state`      | string | `"open"` or `"closed"`                                    |
| `open_time`  | float  | Seconds the door has been (or was) open                   |
| `is_warning` | bool   | `true` if open duration exceeds `warning_time`            |

## Data Capture Behavior

This sensor is designed to work with the **Viam Data Manager** and uses smart filtering to avoid storing redundant data:

1. **Door opens** — `Readings` returns data on every capture cycle (state, duration, warning status).
2. **Door closes** — The first `Readings` call returns the final state with the total open duration.
3. **Door stays closed** — Subsequent `Readings` calls return a gRPC `FailedPrecondition` error (`ErrNoCaptureToStore`), signaling the Data Manager to skip storage until the next event.

This means data is only stored when the door is open or on the transition to closed, keeping your dataset focused on meaningful events.
