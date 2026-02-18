# Model clint:door-monitor:door-monitor

The **Door Monitor** module tracks the state of a door (Open/Closed) using a reed switch connected to a Board component's GPIO pin. It provides visual feedback via status lights (Green/Yellow/Red) and logs usage data to the Viam Data Manager.

## Features

- **Door State Monitoring**: Detects if door is open or closed via GPIO.
- **Visual Feedback**:
  - **Green**: Door Closed.
  - **Yellow**: Door Open (normal duration).
  - **Red**: Door Open too long (Warning).
- **Data Logging**: Automatically captures tabular data events when the door opens and closes.
- **Status API**: `DoCommand` returns real-time status and duration.

## Configuration

Add the `door-monitor` component to your machine configuration.

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
  "type": "component",
  "details": {
    "board_name": "local-board",
    "sensor_pin": "8",
    "sensor_type": "NO",
    "green_light_pin": "10",
    "yellow_light_pin": "12",
    "red_light_pin": "16",
    "warning_time": 60
  }
}
```

## Data Capturing

The module integrates with the **Viam Data Manager**.

- It triggers a data capture **when the door opens** and **when the door closes**.
- Ensure a **Data Manager** service is configured on your machine to collect these events.

## DoCommand

You can query the current status of the door monitor using `DoCommand`.

### Command: `status`

Input:

```json
{ "command": "status" }
```

Output:

```json
{
  "state": "open", // "open" or "closed"
  "open_time": 15.5, // Duration in seconds if open (or duration of last open if closed)
  "is_warning": false // True if open longer than warning_time
}
```
