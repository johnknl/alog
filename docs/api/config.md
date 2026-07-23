# Configuration

### Rotation and Retention

| `MaxDiskSize` | `MaxSegmentSize` | `MaxSegments` | Behavior |
| --- | --- | --- | --- |
| `> 0` | `> 0` | `0` | Byte-budget retention with size-based rotation |
| `> 0` | `> 0` | `> 0` | Byte-budget retention plus segment-count guard |
| `<= 0` | `> 0` | `0` | Size-based rotation with unbounded retained bytes |
| `<= 0` | `> 0` | `1` | Single bounded segment |
| `<= 0` | `<= 0` | `0` | Fully unbounded segment chain |

