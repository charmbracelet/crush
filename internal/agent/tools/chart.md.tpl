Render a chart in the terminal to visualize numeric data. The chart is rendered visually in the UI; the text response you receive is just a short confirmation summary (e.g. "Revenue trend (30 points)"). This is expected. Do not attempt to interpret or re-render the chart from the response text.

## Chart types

- **line**: Plot one or more data series as lines. Good for trends, time series, and continuous measurements.
- **bar**: Plot discrete values as bars. For a simple bar chart, pass a single data array. For a stacked bar chart (showing part-of-whole composition), pass multiple data arrays (one per layer) and provide `series_labels` to name each segment.
- **heatmap**: Plot a 2D matrix of values as a color-mapped grid. Good for correlation matrices, activity grids, and 2D density distributions. Pass data as a 2D array where each inner array is a **row** (not a series). For example, a 5x5 correlation matrix has 5 rows of 5 values each. Labels are used for both the x and y axes.

## Parameters

- **type**: "line" or "bar" (default: "line")
- **data**: Array of series. For line charts, each inner array is one series (e.g. `[[1,4,9,16]]` for a single series, `[[1,2,3],[3,2,1]]` for two). For bar charts, use a single series `[[10,25,40]]`.
- **labels**: Bar labels for bar charts, or x-axis tick labels for line charts.
- **series_labels**: Names for each data layer in stacked bar charts (e.g. ["Desktop", "Mobile", "API"]). Each label gets a colored swatch in the legend.
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

Stacked bar chart showing composition:
```json
{
  "type": "bar",
  "data": [[45, 30, 60], [25, 40, 20], [10, 15, 10]],
  "labels": ["Q1", "Q2", "Q3"],
  "series_labels": ["Desktop", "Mobile", "API"],
  "title": "Revenue by Channel",
  "y_label": "Revenue ($K)"
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

Heatmap of a correlation matrix:
```json
{
  "type": "heatmap",
  "data": [[1.0, 0.8, 0.3], [0.8, 1.0, 0.5], [0.3, 0.5, 1.0]],
  "labels": ["A", "B", "C"],
  "title": "Feature Correlation",
  "x_label": "Feature",
  "y_label": "Feature"
}
```
