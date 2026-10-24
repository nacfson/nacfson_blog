import React from 'react';
import { render, screen, fireEvent } from '@testing-library/react';
import { describe, it, expect } from 'vitest';
import { MarkdownDispatcher } from '../MarkdownDispatcher';

const SAMPLE_MD = `
## 1. Overview

\`\`\`blogPicture
{
  "src": "/content/assets/cilium-vault-architecture.svg",
  "alt": "Cilium Architecture",
  "caption": "Figure 1: Wire-level proxy architecture"
}
\`\`\`

\`\`\`blogIframeTabs
{
  "mountStrategy": "click-or-center",
  "height": "480px",
  "tabs": [
    {
      "id": "sim",
      "label": "Packet Simulator",
      "src": "/content/demos/hubble-flow-sim.html",
      "poster": "/content/assets/wire-proxy-poster.svg",
      "caption": "Interactive flow simulator"
    }
  ]
}
\`\`\`

\`\`\`benchmarkTable
{
  "caption": "Table 1: Metrics",
  "columns": ["Metric", "Standard", "Ours"],
  "rows": [["Plaintext Secrets", "4", "**0**"]]
}
\`\`\`
`;

describe('MarkdownDispatcher', () => {
  it('renders headings, blogPicture, blogIframeTabs poster, and benchmarkTable', () => {
    render(<MarkdownDispatcher markdown={SAMPLE_MD} />);
    expect(screen.getByText('1. Overview')).toBeDefined();
    expect(screen.getByText(/Figure 1: Wire-level proxy architecture/i)).toBeDefined();
    expect(screen.getByText('Table 1: Metrics')).toBeDefined();

    // In click-or-center mode, iframe is not mounted until Launch button is clicked
    expect(document.querySelector('iframe')).toBeNull();
    const launchBtn = screen.getByRole('button', { name: /Launch Interactive Demo/i });
    fireEvent.click(launchBtn);

    const iframe = document.querySelector('iframe');
    expect(iframe).not.toBeNull();
    expect(iframe?.getAttribute('sandbox')).toBe('allow-scripts');
    expect(iframe?.getAttribute('src')).toBe('/content/demos/hubble-flow-sim.html');
  });

  it('renders inline fallback banner when custom block JSON is malformed', () => {
    const broken = "```blogIframeTabs\n{ bad json \n```";
    render(<MarkdownDispatcher markdown={broken} />);
    expect(screen.getByText(/Invalid blogIframeTabs JSON block/i)).toBeDefined();
  });

  it('renders scrollHero and benchmarkChart blocks', () => {
    const md = `
\`\`\`scrollHero
{
  "ariaLabel": "Test Hero",
  "baseImage": "/content/assets/hero-bg-sky.svg",
  "layers": [
    { "src": "/content/assets/hero-layer-1.svg", "depth": 0.25 }
  ],
  "controls": [
    { "id": "speed", "label": "Flow Speed", "min": 0, "max": 2, "step": 0.1, "default": 1 }
  ]
}
\`\`\`

\`\`\`benchmarkChart
{
  "title": "Query Latency (ms)",
  "unit": "ms",
  "series": [
    { "name": "Direct DB", "value": 0.42 },
    { "name": "Proxy DB", "value": 0.58 }
  ]
}
\`\`\`
`;
    render(<MarkdownDispatcher markdown={md} />);
    expect(screen.getByLabelText('Test Hero')).toBeDefined();
    expect(screen.getByText('Query Latency (ms)')).toBeDefined();
    expect(screen.getByText('Direct DB')).toBeDefined();
    expect(screen.getByText('Proxy DB')).toBeDefined();
  });

  it('does not prematurely close a fenced block when inline triple backticks appear inside a JSON string', () => {
    const md = `
\`\`\`blogPicture
{
  "lightSrc": "/content/assets/cilium-vault-architecture.svg",
  "alt": "Diagram",
  "caption": "Rendered via \`\`\`blogPicture with locked aspect ratio"
}
\`\`\`

\`\`\`blogIframeTabs
{
  "tabs": [
    {
      "label": "Live Demo Tab",
      "panels": [
        {
          "type": "iframe",
          "src": "/content/demos/hubble-flow-sim.html",
          "poster": "/content/assets/wire-proxy-poster.svg",
          "interaction": "click-or-center"
        }
      ]
    }
  ]
}
\`\`\`
`;
    render(<MarkdownDispatcher markdown={md} />);
    expect(screen.getByText(/Rendered via \`\`\`blogPicture/)).toBeDefined();
    expect(screen.getByText('Live Demo Tab')).toBeDefined();
    expect(screen.getByRole('button', { name: /Launch Interactive Demo/i })).toBeDefined();
  });
});
