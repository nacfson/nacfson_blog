export type Topic = 'cloud-native' | 'zero-trust' | 'ai-agents' | 'open-source';

export interface PostSummary {
  slug: string;
  title: string;
  date: string;
  topic: Topic;
  project?: string;
  summary: string;
  tags: string[];
  readTimeMinutes: number;
}

export interface Post extends PostSummary {
  markdownBody: string;
}

// Custom Markdown Blocks
export interface ScrollHeroLayer {
  src: string;
  depth?: number;
  role?: string;
  fadeOnScroll?: boolean;
  alt?: string;
  parallaxSpeed?: number;
  initialOffsetY?: number;
}

export interface ScrollHeroControl {
  id: string;
  label: string;
  min: number;
  max: number;
  step: number;
  default: number;
}

export interface ScrollHeroData {
  ariaLabel?: string;
  scrollHint?: string;
  aspectRatio?: string;
  heightVh?: number;
  baseImage: string;
  layers: ScrollHeroLayer[];
  controls?: ScrollHeroControl[];
}

export interface BlogPictureData {
  src?: string;
  lightSrc?: string;
  darkSrc?: string;
  alt?: string;
  caption?: string;
  aspectRatio?: string;
  maxWidth?: string;
}

export interface IframePanel {
  type?: string;
  src: string;
  poster?: string;
  interaction?: 'click-or-center' | 'eager';
  title?: string;
}

export interface IframeTab {
  id?: string;
  label: string;
  captionTitle?: string;
  captionDescription?: string;
  caption?: string;
  src?: string;
  poster?: string;
  panels?: IframePanel[];
  cta?: {
    label: string;
    url: string;
  };
}

export interface BlogIframeTabsData {
  variant?: string;
  ariaLabel?: string;
  mountStrategy?: 'click-or-center' | 'eager';
  height?: string;
  tabs: IframeTab[];
}

export interface BenchmarkSeriesItem {
  name: string;
  value: number;
  color?: string;
}

export interface BenchmarkTab {
  id: string;
  label: string;
  unit?: string;
  series: BenchmarkSeriesItem[];
}

export interface BenchmarkChartData {
  title: string;
  subtitle?: string;
  unit?: string;
  tabs?: BenchmarkTab[];
  series?: BenchmarkSeriesItem[];
}

export interface BenchmarkTableRowObj {
  metric: string;
  values: string[];
}

export interface BenchmarkTableData {
  title?: string;
  caption?: string;
  columns: string[];
  highlightColumn?: number;
  rows: (string[] | BenchmarkTableRowObj)[];
}
