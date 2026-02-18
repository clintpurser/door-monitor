# Door Monitor Module

This Viam module monitors the state of a door using a reed switch connected to a Board component's GPIO pin.

It provides:

- **Real-time Status**: Tracks if the door is Open or Closed.
- **Visual Feedback**: Controls GPIO-connected lights (Green/Yellow/Red) to indicate status and warnings.
- **Data Logging**: Automatically logs tabular data events to the Viam Data Manager when the door opens or closes.
- **Warning System**: Triggers a "Red" warning state if the door remains open longer than a configurable threshold (default 60s).

## Models

- [`clint:door-monitor:door-monitor`](clint_door-monitor_door-monitor.md) - The main component model provided by this module.

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
  "type": "component",
  "details": {
    "board_name": "local",
    "sensor_pin": "8",
    "green_light_pin": "10",
    "yellow_light_pin": "12",
    "red_light_pin": "16"
  }
}
```
