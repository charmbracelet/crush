Render a chart in the terminal to visualize numeric data. The chart is rendered visually in the UI; the text response you receive is just a short confirmation summary (e.g. "Revenue trend (30 points)"). This is expected. Do not attempt to interpret or re-render the chart from the response text.

## Chart types

- **line**: Plot one or more data series as lines. Good for trends, time series, and continuous measurements.
- **bar**: Plot discrete values as horizontal bars. Good for comparisons across categories.

## Parameters

- **type**: "line" or "bar" (default: "line")
- **data**: Array of series. For line charts, each inner array is one series (e.g. `[[1,4,9,16]]` for a single series, `[[1,2,3],[3,2,1]]` for two). For bar charts, use a single series `[[10,25,40]]`.
- **labels**: Bar labels for bar charts, or x-axis tick labels for line charts.
- **title**: Chart title shown in the tool call header.
- **x_label**: X-axis label (e.g. "Time", "Month"). Provide this when the x-axis represents a meaningful dimension.
- **y_label**: Y-axis label (e.g. "Revenue ($)", "Count", "Temperature (°C)"). Provide this when the y-axis represents a meaningful dimension.
- **interpolation**: "linear" (default, straight lines between points) or "step" (flat segments with vertical jumps). Use "step" for discrete or quantized data that holds a value then jumps, like state changes, status codes, or step functions. Use "linear" for continuous data like trends or measurements.
- **width**: Preferred width in columns (the renderer adjusts to fit the terminal).
- **height**: Chart height in rows for line charts (default: 15).

## Examples

Line chart of daily revenue:
```json
{
  "type": "line",
  "data": [[1200, 1500, 1100, 1800, 2200, 1900, 2400]],
  "labels": ["Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"],
  "title": "Daily Revenue",
  "x_label": "Day",
  "y_label": "Revenue ($)"
}
```

Bar chart comparing categories:
```json
{
  "type": "bar",
  "data": [[45, 72, 28, 61]],
  "labels": ["Python", "Go", "Rust", "TypeScript"],
  "title": "Language Popularity",
  "y_label": "Count"
}
```

Step chart for discrete state changes:
```json
{
  "type": "line",
  "data": [[1, 1, 3, 3, 3, 2, 2, 5, 5]],
  "interpolation": "step",
  "title": "System State Over Time",
  "x_label": "Time",
  "y_label": "State"
}
```

Multi-series line chart:
```json
{
  "type": "line",
  "data": [[10, 20, 30, 40], [40, 30, 20, 10]],
  "title": "Crossing Trends",
  "x_label": "Day",
  "y_label": "Value"
}
```
