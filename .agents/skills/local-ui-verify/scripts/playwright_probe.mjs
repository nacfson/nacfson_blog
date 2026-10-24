#!/usr/bin/env node
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { pathToFileURL } from "node:url";

async function loadPlaywright() {
  try {
    return await import("playwright");
  } catch {
    const npxRoot = path.join(os.homedir(), ".npm", "_npx");
    if (fs.existsSync(npxRoot)) {
      for (const entry of fs.readdirSync(npxRoot)) {
        const candidate = path.join(npxRoot, entry, "node_modules", "playwright", "index.mjs");
        if (fs.existsSync(candidate)) {
          return await import(pathToFileURL(candidate).href);
        }
      }
    }
    throw new Error("Playwright module not found.");
  }
}

function findSystemChromiumExecutable() {
  const candidates = [
    process.env.PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH,
    "/usr/bin/google-chrome",
    "/usr/bin/google-chrome-stable",
    "/usr/bin/chromium",
    "/usr/bin/chromium-browser",
    "/usr/bin/brave-browser",
    "/opt/brave.com/brave/brave",
  ].filter(Boolean);

  for (const p of candidates) {
    if (fs.existsSync(p)) {
      return p;
    }
  }
  return undefined;
}

function parseArgs(argv) {
  const args = {
    baseUrl: "http://127.0.0.1:8080",
    profile: "blog",
    artifactDir: "/tmp/local-ui-verify-artifacts",
    routes: [],
  };
  for (let i = 2; i < argv.length; i++) {
    const a = argv[i];
    if (a === "--base-url" && argv[i + 1]) args.baseUrl = argv[++i].replace(/\/$/, "");
    else if (a === "--profile" && argv[i + 1]) args.profile = argv[++i];
    else if (a === "--artifact-dir" && argv[i + 1]) args.artifactDir = argv[++i];
    else if (a === "--routes" && argv[i + 1]) {
      args.routes = argv[++i]
        .split(",")
        .map((r) => r.trim())
        .filter(Boolean);
    }
  }
  return args;
}

async function launchBrowser(chromium) {
  const execPath = findSystemChromiumExecutable();
  if (execPath) {
    try {
      return await chromium.launch({ executablePath: execPath, headless: true });
    } catch {
      // Fallback to Playwright managed browser
    }
  }
  return await chromium.launch({ headless: true });
}

async function run() {
  const args = parseArgs(process.argv);
  fs.mkdirSync(args.artifactDir, { recursive: true });

  const { chromium } = await loadPlaywright();
  const browser = await launchBrowser(chromium);

  const consoleErrors = [];
  const pageErrors = [];
  const failedRequests = [];
  const assertions = [];
  const screenshots = [];

  const recordAssertion = (name, passed, detail = "") => {
    assertions.push({ name, passed: Boolean(passed), detail });
  };

  try {
    const context = await browser.newContext({ viewport: { width: 1440, height: 900 } });
    const page = await context.newPage();

    page.on("console", (msg) => {
      if (msg.type() === "error") {
        consoleErrors.push(msg.text());
      }
    });
    page.on("pageerror", (err) => {
      pageErrors.push(String(err));
    });
    page.on("response", (res) => {
      if (res.status() >= 400 && !res.url().includes("/favicon.ico")) {
        failedRequests.push(`${res.status()} ${res.url()}`);
      }
    });

    if (args.profile === "blog") {
      // 1. Home Desktop
      await page.goto(`${args.baseUrl}/`, { waitUntil: "networkidle" });
      await page.waitForSelector("h1", { timeout: 10000 });

      const bg = await page.evaluate(() => getComputedStyle(document.body).backgroundColor);
      recordAssertion(
        "home_light_theme_background",
        bg === "rgb(248, 250, 252)",
        `body background-color=${bg}`
      );

      const bodyText = await page.locator("body").innerText();
      recordAssertion(
        "home_competency_matrix_rendered",
        bodyText.includes("Technical Competency Matrix") &&
          bodyText.includes("CNCF & Kubernetes Platform Engineering"),
        "Competency matrix & domain cards visible"
      );
      recordAssertion(
        "home_seed_post_card_rendered",
        bodyText.includes("Zero-Plaintext K3s: Cilium eBPF, OpenBao & Go Wire Proxies"),
        "Seed article card rendered in feed"
      );

      const homeDesktopShot = path.join(args.artifactDir, "home-desktop.png");
      await page.screenshot({ path: homeDesktopShot, fullPage: true });
      screenshots.push(homeDesktopShot);

      // 2. Home Mobile
      await page.setViewportSize({ width: 390, height: 844 });
      const homeMobileShot = path.join(args.artifactDir, "home-mobile.png");
      await page.screenshot({ path: homeMobileShot, fullPage: true });
      screenshots.push(homeMobileShot);

      // 3. Article Reader Desktop + Custom Blocks + Interactive Sandbox
      await page.setViewportSize({ width: 1440, height: 900 });
      await page.goto(`${args.baseUrl}/post/?slug=cilium-zero-plaintext-k3s`, {
        waitUntil: "networkidle",
      });
      await page.waitForSelector("article", { timeout: 10000 });

      const articleText = (await page.locator("article").innerText()).toLowerCase();
      recordAssertion(
        "article_custom_blocks_rendered",
        articleText.includes("zero-plaintext cluster architecture") &&
          articleText.includes("cluster networking & wire proxy performance") &&
          articleText.includes("tenant security & resource contract matrix"),
        "ScrollHero, BlogPicture, BenchmarkChart, and BenchmarkTable rendered"
      );

      const articleDesktopShot = path.join(args.artifactDir, "article-desktop.png");
      await page.screenshot({ path: articleDesktopShot, fullPage: true });
      screenshots.push(articleDesktopShot);

      const launchBtn = page.getByRole("button", { name: /Launch Interactive Demo/i });
      if ((await launchBtn.count()) > 0) {
        await launchBtn.first().click();
      }
      const iframe = page.locator('iframe[sandbox="allow-scripts"]');
      await iframe.first().waitFor({ state: "visible", timeout: 5000 });
      const iframeSrc = await iframe.first().getAttribute("src");
      recordAssertion(
        "article_sandboxed_iframe_mounted",
        Boolean(iframeSrc && iframeSrc.includes("/content/demos/hubble-flow-sim.html")),
        `iframe src=${iframeSrc}`
      );
      const demoShot = path.join(args.artifactDir, "article-interactive-demo.png");
      await page.screenshot({ path: demoShot, fullPage: true });
      screenshots.push(demoShot);
    } else {
      const targetRoutes = args.routes.length > 0 ? args.routes : ["/"];
      for (const route of targetRoutes) {
        const normalized = route.startsWith("/") ? route : `/${route}`;
        const res = await page.goto(`${args.baseUrl}${normalized}`, { waitUntil: "networkidle" });
        const status = res ? res.status() : 0;
        const textLen = (await page.locator("body").innerText()).trim().length;
        recordAssertion(
          `route_${normalized}_renders`,
          status >= 200 && status < 400 && textLen > 0,
          `HTTP ${status}, textLength=${textLen}`
        );
        const slug = normalized === "/" ? "root" : normalized.replace(/[^a-zA-Z0-9_-]/g, "_");
        const shot = path.join(args.artifactDir, `route-${slug}-desktop.png`);
        await page.screenshot({ path: shot, fullPage: true });
        screenshots.push(shot);
      }
    }

    await context.close();
  } finally {
    await browser.close();
  }

  const allAssertionsPassed = assertions.every((a) => a.passed);
  const allPassed =
    allAssertionsPassed &&
    consoleErrors.length === 0 &&
    pageErrors.length === 0 &&
    failedRequests.length === 0;

  const result = {
    all_passed: allPassed,
    assertions,
    console_errors: consoleErrors,
    page_errors: pageErrors,
    failed_requests: failedRequests,
    screenshots,
  };
  process.stdout.write(JSON.stringify(result, null, 2) + "\n");
  process.exit(allPassed ? 0 : 1);
}

run().catch((err) => {
  process.stdout.write(
    JSON.stringify(
      {
        all_passed: false,
        assertions: [],
        console_errors: [String(err)],
        page_errors: [String(err)],
        failed_requests: [],
        screenshots: [],
      },
      null,
      2
    ) + "\n"
  );
  process.exit(1);
});
