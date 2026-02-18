# Door Monitor Module

This Viam module provides a **sensor component** that monitors the state of a door using a reed switch connected to a Board component's GPIO pin.

It provides:

- **Real-time Status**: Tracks if the door is Open or Closed via the standard `Readings` API.
- **Visual Feedback**: Controls GPIO-connected lights (Green/Yellow/Red) to indicate status and warnings.
- **Smart Data Capture**: Works with the Viam Data Manager to only store data when the door is open or transitioning to closed. Returns `ErrNoCaptureToStore` when the door is idle (closed), keeping your dataset focused on meaningful events.
- **Warning System**: Triggers a "Red" warning state if the door remains open longer than a configurable threshold (default 60s).

## Models

- [`clint:door-monitor:door-monitor`](clint_door-monitor_door-monitor.md) - The sensor component model provided by this module.

## Build

To build the module binary:

```bash
go build -o door-monitor-module .
```

## Configuration

See [`clint_door-monitor_door-monitor.md`](clint_door-monitor_door-monitor.md) for full configuration details and attribute descriptions.

### Quick Example

```json
{
  "name": "my-door-monitor",
  "model": "clint:door-monitor:door-monitor",
  "type": "sensor",
  "namespace": "rdk",
  "attributes": {
    "board_name": "local",
    "sensor_pin": "8",
    "green_light_pin": "10",
    "yellow_light_pin": "12",
    "red_light_pin": "16"
  },
  "depends_on": ["local"]
}
```
