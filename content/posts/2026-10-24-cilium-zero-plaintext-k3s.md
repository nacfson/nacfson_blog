---
title: "Zero-Plaintext K3s: Cilium eBPF, OpenBao & Go Wire Proxies"
slug: "cilium-zero-plaintext-k3s"
date: "2026-10-24"
pillar: "project-updates"
project: "nacfson-pipeline"
tags: ["kubernetes", "cilium", "openbao", "fluxcd", "golang"]
summary: "How we replaced Flannel and plaintext Kubernetes Secrets with Git-vendored Cilium v1.20.3, Hubble Relay observability, and a zero-dependency Go PostgreSQL wire proxy on a single ARM64 K3s cluster."
readTimeMinutes: 8
---

```scrollHero
{
  "ariaLabel": "Interactive sea of clouds and Cilium eBPF cluster topology",
  "scrollHint": "Scroll to explore",
  "aspectRatio": "1600/760",
  "baseImage": "/content/assets/hero-bg-sky.svg",
  "layers": [
    { "src": "/content/assets/hero-layer-1-back-clouds.svg", "depth": 0.25, "role": "background-clouds" },
    { "src": "/content/assets/hero-layer-2-title.svg", "depth": 0.55, "role": "title-mark", "fadeOnScroll": true },
    { "src": "/content/assets/hero-layer-3-front-clouds.svg", "depth": 0.90, "role": "foreground-mist" }
  ],
  "controls": [
    { "id": "flowSpeed", "label": "eBPF Packet Flow", "min": 0.2, "max": 3.0, "step": 0.1, "default": 1.4 },
    { "id": "parallaxDepth", "label": "Scroll Parallax Depth", "min": 0, "max": 100, "step": 5, "default": 65 }
  ]
}
```

## 1. Zero-Plaintext Cluster Architecture

In **`nacfson_pipeline`**, every hosted tenant project (`proj-pn`, `proj-blog`) runs on a single ARM64 K3s node reconciled continuously by **FluxCD**. Instead of scattering database passwords across Kubernetes `Secret` manifests, credentials stay inside **OpenBao 2.0.0** and are injected at the wire protocol level.

```blogPicture
{
  "lightSrc": "/content/assets/cilium-vault-architecture.svg",
  "darkSrc": "/content/assets/cilium-vault-architecture.svg",
  "alt": "End-to-end packet path from Traefik Ingress through Cilium eBPF policies to vault-db-proxy and PostgreSQL 16",
  "caption": "Figure 1: Real SVG diagram (/content/assets/cilium-vault-architecture.svg) rendered via ```blogPicture with locked 1200/520 aspect ratio.",
  "aspectRatio": "1200/520"
}
```

## 2. Interactive HTML + Poster Image Sandbox

Below, `blogIframeTabs` displays a lightweight static image (`/content/assets/wire-proxy-poster.svg`) first and mounts the real standalone interactive HTML file (`/content/demos/hubble-flow-sim.html`) inside an `<iframe>` when clicked (`"interaction": "click-or-center"`):

```blogIframeTabs
{
  "variant": "case-showcase",
  "ariaLabel": "Interactive Kubernetes & Cilium Demos",
  "tabs": [
    {
      "label": "1. Vault Wire-Proxy Handshake (Live HTML)",
      "captionTitle": "PostgreSQL v3.0 Wire Handshake Interception",
      "captionDescription": "Step through how blog-backend authenticates as project:blog:workload:backend without holding the user_blog password in pod memory.",
      "panels": [
        {
          "type": "iframe",
          "src": "/content/demos/hubble-flow-sim.html",
          "poster": "/content/assets/wire-proxy-poster.svg",
          "interaction": "click-or-center",
          "title": "Interactive Vault DB Proxy Handshake"
        }
      ],
      "cta": {
        "label": "Open standalone HTML demo",
        "url": "/content/demos/hubble-flow-sim.html"
      }
    }
  ]
}
```

## 3. Interactive Benchmarks

Moving from standard Kubernetes Secret mounts + Flannel VXLAN to **Cilium eBPF** and **`vault-db-proxy`** kept our p99 query latency under `0.6ms`:

```benchmarkChart
{
  "title": "Cluster Networking & Wire Proxy Performance",
  "subtitle": "Measured on Oracle Cloud ARM64 K3s v1.35.9 (Lower latency & memory is better)",
  "tabs": [
    {
      "id": "latency",
      "label": "DB Query Latency (p99 ms)",
      "unit": "ms",
      "series": [
        { "name": "Direct PostgreSQL 16 Socket", "value": 0.42, "color": "#10b981" },
        { "name": "vault-db-proxy (Go 1.22 Stdlib)", "value": 0.58, "color": "#0284c7" },
        { "name": "External Cloud SQL Proxy", "value": 4.10, "color": "#94a3b8" }
      ]
    },
    {
      "id": "competency",
      "label": "CNCF Competency Milestones (%)",
      "unit": "%",
      "series": [
        { "name": "FluxCD GitOps & Admission Policies", "value": 95, "color": "#0284c7" },
        { "name": "Cilium eBPF & L7 FQDN Policies", "value": 90, "color": "#0ea5e9" },
        { "name": "OpenBao Transit & Wire Proxies", "value": 92, "color": "#6366f1" }
      ]
    }
  ]
}
```

## 4. Architecture Comparison Matrix

```benchmarkTable
{
  "title": "Tenant Security & Resource Contract Matrix",
  "columns": ["Capability", "nacfson_pipeline (K3s)", "Default Helm Cluster", "Managed PaaS"],
  "highlightColumn": 1,
  "rows": [
    { "metric": "DB Credentials in Pod", "values": ["0 Plaintext (vault-db-proxy)", "Plaintext Secret", "Env Var"] },
    { "metric": "Pod Security Standard", "values": ["Restricted (UID 10001)", "Baseline", "Managed"] },
    { "metric": "World Egress Filtering", "values": ["Cilium eBPF FQDN Allowlist", "0.0.0.0/0 Open", "0.0.0.0/0 Open"] }
  ]
}
```
