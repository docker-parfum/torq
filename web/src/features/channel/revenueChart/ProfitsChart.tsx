// https://www.pluralsight.com/guides/using-d3.js-inside-a-react-app
import { useD3 } from "features/charts/useD3";
import { useEffect } from "react";
import { Selection } from "d3";
import ChartCanvas from "features/charts/chartCanvas";
import "features/charts/chart.scss";
import { BarPlot, LinePlot } from "features/charts/charts";
import { selectProfitChartKey } from "../channelSlice";
import { useAppSelector } from "store/hooks";
import { useGetSettingsQuery } from "apiSlice";

type ProfitsChart = {
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  data: any[];
  dashboard?: boolean;
  from: string;
  to: string;
};

function ProfitsChart({ data, dashboard, to, from }: ProfitsChart) {
  let chart: ChartCanvas;
  let currentSize: [number | undefined, number | undefined] = [undefined, undefined];
  const profitKey = useAppSelector(selectProfitChartKey);
  const settings = useGetSettingsQuery();

  // Check and update the chart size if the navigation changes the container size
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const navCheck = (container: Selection<HTMLDivElement, Record<string, never>, HTMLElement, any>) => {
    return () => {
      const boundingBox = container?.node()?.getBoundingClientRect();
      if (currentSize[0] !== boundingBox?.width || currentSize[1] !== boundingBox?.height) {
        chart.resizeChart();
        chart.draw();
        currentSize = [boundingBox?.width, boundingBox?.height];
      }
    };
  };

  // TODO: Change this so that we can update the data without redrawing the entire chart
  const ref = useD3(
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    (container: Selection<HTMLDivElement, Record<string, never>, HTMLElement, any>) => {
      if (dashboard) {
        chart = new ChartCanvas(container, data, {
          from: new Date(from),
          to: new Date(to),
          timezone: settings?.data?.preferredTimezone || "UTC",
          yScaleKey: profitKey.value + "Out",
          rightYScaleKey: profitKey.value + "Out",
          rightYAxisKeys: [profitKey.value + "Out"],
          xAxisPadding: 12,
        });
        chart.plot(BarPlot, {
          id: profitKey.value + "Out",
          key: profitKey.value + "Out",
          legendLabel: profitKey.label + " out",
          barColor: "rgba(186, 147, 250, 0.6)",
          barHoverColor: "rgba(186, 147, 250, 0.8)",
          // areaGradient: ["rgba(133, 196, 255, 0.5)", "rgba(87, 211, 205, 0.5)"],
        });
        chart.draw();
      } else {
        chart = new ChartCanvas(container, data, {
          from: new Date(from),
          to: new Date(to),
          timezone: settings?.data?.preferredTimezone || "UTC",
          yScaleKey: profitKey.value + "Total",
          rightYScaleKey: profitKey.value + "Total",
          rightYAxisKeys: [profitKey.value + "Out", profitKey.value + "In"],
          xAxisPadding: 12,
        });
        chart.plot(BarPlot, {
          id: profitKey.value + "Total",
          key: profitKey.value + "Total",
          legendLabel: profitKey.label + " total",
          // areaGradient: ["rgba(133, 196, 255, 0.5)", "rgba(87, 211, 205, 0.5)"],
          barColor: "rgba(133, 196, 255, 0.5)",
        });
        chart.plot(LinePlot, {
          id: profitKey.value + "Out",
          key: profitKey.value + "Out",
          legendLabel: profitKey.label + " out",
          lineColor: "#BA93FA",
        });
        chart.plot(LinePlot, {
          id: profitKey.value + "In",
          key: profitKey.value + "In",
          legendLabel: profitKey.label + " in",
          lineColor: "#FAAE93",
        });
        chart.draw();
      }

      setInterval(navCheck(container), 200);
    },
    [data, data ? data[0].date : "", data ? data[data.length - 1].date : "", profitKey, settings]
  );

  useEffect(() => {
    return () => {
      if (chart) {
        chart.removeResizeListener();
      }
    };
  }, [data]);

  return <div ref={ref} className={"chart-ref"} />;
}

export default ProfitsChart;
