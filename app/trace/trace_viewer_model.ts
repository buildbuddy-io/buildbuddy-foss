import { getLightMaterialChartColor, getMaterialChartColor, getUniformBrightnessColor } from "../util/color";
import * as constants from "./constants";
import { Profile, TraceEvent, buildThreadTimelines, buildTimeSeries } from "./trace_events";

/**
 * A trace event profile structured for easier rendering in the trace viewer.
 */
export type TraceViewerModel = {
  panels: PanelModel[];
  xMax: number;
};

export type PanelModel = {
  height: number;
  sections: SectionModel[];
};

export type SectionModel = {
  name: string;
  y: number;
  height: number;

  // A section will contain either a list of tracks or a line plot.

  tracks?: TrackModel[];
  linePlot?: LinePlotModel;
};

export type TrackModel = {
  xs: number[];
  widths: number[];
  colors: string[];
  events: TraceEvent[];
};

export type LinePlotModel = {
  xs: number[];
  ys: number[];
  yMax: number;
  darkColor: string;
  lightColor: string;
  unit?: string;
};

export function buildTraceViewerModel(trace: Profile, fitToContent?: boolean): TraceViewerModel {
  let panels = [
    buildEventsPanel(trace.traceEvents, fitToContent),
    buildLinePlotsPanel(trace.traceEvents, fitToContent),
  ];
  // If there is no data available (e.g. the executor doesn't have timeseries
  // recording enabled yet) then the panel will be empty - just remove the panel
  // in this case since we don't handle this empty state in a good way yet.
  panels = panels.filter((panel) => panel.sections.length);

  return {
    panels,
    xMax: computeXMax(panels),
  };
}

function buildEventsPanel(events: TraceEvent[], fitToContent?: boolean): PanelModel {
  const sections: SectionModel[] = [];
  let sectionY = 0;
  const timelines = buildThreadTimelines(events);
  for (const { threadName, events } of timelines) {
    // Don't show threads with no events.
    if (!events.length) continue;

    const tracks: TrackModel[] = [];
    for (const event of events) {
      const { ts, dur, cat, name, depth } = event;

      const track = (tracks[depth] ??= {
        xs: [],
        widths: [],
        colors: [],
        events: [],
      });
      track.xs.push(ts);
      track.widths.push(dur);
      track.colors.push(getUniformBrightnessColor(`${cat}#${name}`));
      track.events.push(event);
    }

    const sectionHeight =
      constants.SECTION_LABEL_HEIGHT +
      constants.SECTION_LABEL_PADDING_BOTTOM +
      constants.SECTION_PADDING_BOTTOM +
      tracks.length * constants.TRACK_HEIGHT +
      (tracks.length - 1) * constants.TRACK_VERTICAL_GAP;

    sections.push({
      name: threadName,
      y: sectionY,
      height: sectionHeight,
      tracks,
    });
    sectionY += sectionHeight;
  }

  return {
    height: fitToContent
      ? constants.TIMESTAMP_HEADER_SIZE + sectionY + constants.BOTTOM_CONTROLS_HEIGHT + constants.SCROLLBAR_SIZE
      : constants.EVENTS_PANEL_HEIGHT,
    sections,
  };
}

function buildLinePlotsPanel(events: TraceEvent[], fitToContent?: boolean): PanelModel {
  const timeSeries = buildTimeSeries(events);
  let sectionY = 0;
  let index = 0;
  const sectionHeight =
    constants.SECTION_LABEL_HEIGHT +
    constants.SECTION_LABEL_PADDING_BOTTOM +
    constants.TIME_SERIES_HEIGHT +
    constants.SECTION_PADDING_BOTTOM;
  const sections: SectionModel[] = [];
  for (const { name, events, unit } of timeSeries) {
    const xs: number[] = [];
    const ys: number[] = [];
    let yMax = 0;
    for (const event of events) {
      const x = event.ts;
      const y = Number(event.value);
      if (isNaN(y)) continue;
      xs.push(x);
      ys.push(y);
      if (y > yMax) yMax = y;
    }
    sections.push({
      name: name,
      y: sectionY,
      height: sectionHeight,
      linePlot: {
        xs,
        ys,
        yMax,
        darkColor: getMaterialChartColor(index),
        lightColor: getLightMaterialChartColor(index),
        unit,
      },
    });
    sectionY += sectionHeight;
    index++;
  }

  return {
    height: fitToContent
      ? constants.TIMESTAMP_HEADER_SIZE + sectionY + constants.BOTTOM_CONTROLS_HEIGHT + constants.SCROLLBAR_SIZE
      : constants.LINE_PLOTS_PANEL_HEIGHT,
    sections,
  };
}

/**
 * Returns the max x-coordinate needed to render all of the data in the model.
 */
function computeXMax(panels: PanelModel[]): number {
  let width = 0;
  for (const panel of panels) {
    for (const section of panel.sections) {
      for (const track of section.tracks ?? []) {
        if (!track.xs.length) continue;
        const lastIdx = track.xs.length - 1;
        const trackWidth = track.xs[lastIdx] + track.widths[lastIdx];
        if (trackWidth > width) width = trackWidth;
      }
      if (section.linePlot) {
        if (!section.linePlot.xs.length) continue;
        const lastX = section.linePlot.xs[section.linePlot.xs.length - 1];
        if (lastX > width) width = lastX;
      }
    }
  }
  return width;
}

/**
 * Returns the total scrollable height of all panel contents, including the
 * header and footer.
 */
export function panelScrollHeight(panel: PanelModel): number {
  let height = constants.TIMESTAMP_HEADER_SIZE + constants.BOTTOM_CONTROLS_HEIGHT;
  if (!panel.sections.length) height;

  const lastSection = panel.sections[panel.sections.length - 1];
  height += lastSection.y + lastSection.height;

  return height;
}
