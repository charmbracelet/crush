// Get all charmtone colors once from computed styles
const rootStyles = getComputedStyle(document.documentElement);
const colors = {
  charple: rootStyles.getPropertyValue("--charple").trim(),
  cherry: rootStyles.getPropertyValue("--cherry").trim(),
  julep: rootStyles.getPropertyValue("--julep").trim(),
  urchin: rootStyles.getPropertyValue("--urchin").trim(),
  butter: rootStyles.getPropertyValue("--butter").trim(),
  squid: rootStyles.getPropertyValue("--squid").trim(),
  pepper: rootStyles.getPropertyValue("--pepper").trim(),
  tuna: rootStyles.getPropertyValue("--tuna").trim(),
  uni: rootStyles.getPropertyValue("--uni").trim(),
  coral: rootStyles.getPropertyValue("--coral").trim(),
  violet: rootStyles.getPropertyValue("--violet").trim(),
  malibu: rootStyles.getPropertyValue("--malibu").trim(),
};

const easeDuration = 500;
const easeType = "easeOutQuart";

// Helper functions
function formatNumber(n) {
  return new Intl.NumberFormat().format(Math.round(n));
}

function formatCompact(n) {
  if (n >= 1000000) return (n / 1000000).toFixed(1) + "M";
  if (n >= 1000) return (n / 1000).toFixed(1) + "k";
  return Math.round(n).toString();
}

function formatCost(n) {
  return "$" + n.toFixed(2);
}

function formatTime(ms) {
  if (ms < 1000) return Math.round(ms) + "ms";
  return (ms / 1000).toFixed(1) + "s";
}

const charpleColor = { r: 107, g: 80, b: 255 };
const tunaColor = { r: 255, g: 109, b: 170 };

function interpolateColor(ratio, alpha = 1) {
  const r = Math.round(charpleColor.r + (tunaColor.r - charpleColor.r) * ratio);
  const g = Math.round(charpleColor.g + (tunaColor.g - charpleColor.g) * ratio);
  const b = Math.round(charpleColor.b + (tunaColor.b - charpleColor.b) * ratio);
  if (alpha < 1) {
    return `rgba(${r}, ${g}, ${b}, ${alpha})`;
  }
  return `rgb(${r}, ${g}, ${b})`;
}

function getTopItemsWithOthers(items, countKey, labelKey, topN = 10) {
  const topItems = items.slice(0, topN);
  const otherItems = items.slice(topN);
  const otherCount = otherItems.reduce((sum, item) => sum + item[countKey], 0);
  const displayItems = [...topItems];
  if (otherItems.length > 0) {
    const otherItem = { [countKey]: otherCount, [labelKey]: "others" };
    displayItems.push(otherItem);
  }
  return displayItems;
}

// Populate summary cards
document.getElementById("total-sessions").textContent = formatNumber(
  stats.total.total_sessions,
);
document.getElementById("total-messages").textContent = formatCompact(
  stats.total.total_messages,
);
document.getElementById("total-tokens").textContent = formatCompact(
  stats.total.total_tokens,
);
document.getElementById("total-cost").textContent = formatCost(
  stats.total.total_cost,
);
document.getElementById("avg-tokens").innerHTML =
  '<span title="Average">x̅</span> ' +
  formatCompact(stats.total.avg_tokens_per_session);
document.getElementById("avg-response").innerHTML =
  '<span title="Average">x̅</span> ' + formatTime(stats.avg_response_time_ms);

// Chart defaults
Chart.defaults.color = colors.squid;
Chart.defaults.borderColor = colors.squid;

if (stats.recent_activity?.length > 0) {
  new Chart(document.getElementById("recentActivityChart"), {
    type: "bar",
    data: {
      labels: stats.recent_activity.map((d) => d.day),
      datasets: [
        {
          label: "Sessions",
          data: stats.recent_activity.map((d) => d.session_count),
          backgroundColor: colors.charple,
          borderRadius: 4,
          yAxisID: "y",
        },
        {
          label: "Tokens (K)",
          data: stats.recent_activity.map((d) => d.total_tokens / 1000),
          backgroundColor: colors.julep,
          borderRadius: 4,
          yAxisID: "y1",
        },
      ],
    },
    options: {
      responsive: true,
      maintainAspectRatio: false,
      animation: { duration: 800, easing: easeType },
      interaction: { mode: "index", intersect: false },
      scales: {
        y: { position: "left", title: { display: true, text: "Sessions" } },
        y1: {
          position: "right",
          title: { display: true, text: "Tokens (K)" },
          grid: { drawOnChartArea: false },
        },
      },
    },
  });
}

// Heatmap (Hour × Day of Week) - Bubble Chart
const dayLabels = ["Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"];

let maxCount =
  stats.hour_day_heatmap?.length > 0
    ? Math.max(...stats.hour_day_heatmap.map((h) => h.session_count))
    : 0;
if (maxCount === 0) maxCount = 1;
const scaleFactor = 20 / Math.sqrt(maxCount);

if (stats.hour_day_heatmap?.length > 0) {
  new Chart(document.getElementById("heatmapChart"), {
    type: "bubble",
    data: {
      datasets: [
        {
          label: "Sessions",
          data: stats.hour_day_heatmap
            .filter((h) => h.session_count > 0)
            .map((h) => ({
              x: h.hour,
              y: h.day_of_week,
              r: Math.sqrt(h.session_count) * scaleFactor,
              count: h.session_count,
            })),
          backgroundColor: (ctx) => {
            const count =
              ctx.raw?.count || ctx.dataset.data[ctx.dataIndex]?.count || 0;
            const ratio = count / maxCount;
            return interpolateColor(ratio);
          },
          borderWidth: 0,
        },
      ],
    },
    options: {
      responsive: true,
      maintainAspectRatio: false,
      animation: false,
      scales: {
        x: {
          min: 0,
          max: 23,
          grid: { display: false },
          title: { display: true, text: "Hour of Day" },
          ticks: {
            stepSize: 1,
            callback: (v) => (Number.isInteger(v) ? v : ""),
          },
        },
        y: {
          min: 0,
          max: 6,
          reverse: true,
          grid: { display: false },
          title: { display: true, text: "Day of Week" },
          ticks: { stepSize: 1, callback: (v) => dayLabels[v] || "" },
        },
      },
      plugins: {
        legend: { display: false },
        tooltip: {
          callbacks: {
            label: (ctx) =>
              dayLabels[ctx.raw.y] +
              " " +
              ctx.raw.x +
              ":00 - " +
              ctx.raw.count +
              " sessions",
          },
        },
      },
    },
  });
}

if (stats.tool_usage?.length > 0) {
  const displayTools = getTopItemsWithOthers(
    stats.tool_usage,
    "call_count",
    "tool_name",
  );
  const maxValue = Math.max(...displayTools.map((t) => t.call_count));
  new Chart(document.getElementById("toolChart"), {
    type: "bar",
    data: {
      labels: displayTools.map((t) => t.tool_name),
      datasets: [
        {
          label: "Calls",
          data: displayTools.map((t) => t.call_count),
          backgroundColor: (ctx) => {
            const value = ctx.raw;
            const ratio = value / maxValue;
            return interpolateColor(ratio);
          },
          borderRadius: 4,
        },
      ],
    },
    options: {
      indexAxis: "y",
      responsive: true,
      maintainAspectRatio: false,
      animation: { duration: easeDuration, easing: easeType },
      plugins: { legend: { display: false } },
    },
  });
}

// Token Distribution Pie
new Chart(document.getElementById("tokenPieChart"), {
  type: "doughnut",
  data: {
    labels: ["Prompt Tokens", "Completion Tokens"],
    datasets: [
      {
        data: [
          stats.total.total_prompt_tokens,
          stats.total.total_completion_tokens,
        ],
        backgroundColor: [colors.charple, colors.julep],
        borderWidth: 0,
      },
    ],
  },
  options: {
    responsive: true,
    maintainAspectRatio: false,
    animation: { duration: easeDuration, easing: easeType },
    plugins: {
      legend: { position: "bottom" },
    },
  },
});

// Model Usage Chart (horizontal bar)
if (stats.usage_by_model?.length > 0) {
  const displayModels = getTopItemsWithOthers(
    stats.usage_by_model,
    "message_count",
    "model",
  );
  const maxModelValue = Math.max(...displayModels.map((m) => m.message_count));
  new Chart(document.getElementById("modelChart"), {
    type: "bar",
    data: {
      labels: displayModels.map((m) =>
        m.provider ? `${m.model} (${m.provider})` : m.model,
      ),
      datasets: [
        {
          label: "Messages",
          data: displayModels.map((m) => m.message_count),
          backgroundColor: (ctx) => {
            const value = ctx.raw;
            const ratio = value / maxModelValue;
            return interpolateColor(ratio);
          },
          borderRadius: 4,
        },
      ],
    },
    options: {
      indexAxis: "y",
      responsive: true,
      maintainAspectRatio: false,
      animation: { duration: easeDuration, easing: easeType },
      plugins: { legend: { display: false } },
    },
  });
}

if (stats.usage_by_model?.length > 0) {
  const providerData = stats.usage_by_model.reduce((acc, m) => {
    acc[m.provider] = (acc[m.provider] || 0) + m.message_count;
    return acc;
  }, {});
  const providerColors = [
    colors.malibu,
    colors.charple,
    colors.violet,
    colors.tuna,
    colors.coral,
    colors.uni,
  ];
  new Chart(document.getElementById("providerPieChart"), {
    type: "doughnut",
    data: {
      labels: Object.keys(providerData),
      datasets: [
        {
          data: Object.values(providerData),
          backgroundColor: Object.keys(providerData).map(
            (_, i) => providerColors[i % providerColors.length],
          ),
          borderWidth: 0,
        },
      ],
    },
    options: {
      responsive: true,
      maintainAspectRatio: false,
      animation: { duration: easeDuration, easing: easeType },
      plugins: {
        legend: { position: "bottom" },
      },
    },
  });
}

// Daily Usage Table
const tableBody = document.querySelector("#daily-table tbody");
if (stats.usage_by_day?.length > 0) {
  const fragment = document.createDocumentFragment();
  stats.usage_by_day.slice(0, 30).forEach((d) => {
    const row = document.createElement("tr");
    row.innerHTML = `<td>${d.day}</td><td>${d.session_count}</td><td>${formatNumber(
      d.prompt_tokens,
    )}</td><td>${formatNumber(
      d.completion_tokens,
    )}</td><td>${formatNumber(d.total_tokens)}</td><td>${formatCost(
      d.cost,
    )}</td>`;
    fragment.appendChild(row);
  });
  tableBody.appendChild(fragment);
}
