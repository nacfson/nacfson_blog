#!/usr/bin/env node
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { pathToFileURL } from 'node:url';

async function loadPlaywright() {
  try {
    return await import('playwright');
  } catch {
    const npxRoot = path.join(os.homedir(), '.npm', '_npx');
    if (fs.existsSync(npxRoot)) {
      for (const entry of fs.readdirSync(npxRoot)) {
        const candidate = path.join(npxRoot, entry, 'node_modules', 'playwright', 'index.mjs');
        if (fs.existsSync(candidate)) {
          return await import(pathToFileURL(candidate).href);
        }
      }
    }
    throw new Error('Playwright module not found.');
  }
}

function findSystemChromiumExecutable() {
  const candidates = [
    process.env.PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH,
    '/usr/bin/google-chrome',
    '/usr/bin/google-chrome-stable',
    '/usr/bin/chromium',
    '/usr/bin/chromium-browser',
    '/usr/bin/brave-browser',
    '/opt/brave.com/brave/brave',
  ].filter(Boolean);

  for (const p of candidates) {
    if (fs.existsSync(p)) return p;
  }
  return undefined;
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

async function main() {
  const baseUrl = process.argv[2];
  const screenshotPath = process.argv[3];
  if (!baseUrl || !screenshotPath) {
    throw new Error('Usage: e2e_playwright.mjs <baseUrl> <screenshotPath>');
  }

  const { chromium } = await loadPlaywright();
  const browser = await launchBrowser(chromium);
  const consoleErrors = [];
  const pageErrors = [];

  try {
    const context = await browser.newContext({ viewport: { width: 1440, height: 900 } });
    const page = await context.newPage();

    page.on('console', (msg) => {
      if (msg.type() === 'error') {
        consoleErrors.push(msg.text());
      }
    });
    page.on('pageerror', (err) => {
      pageErrors.push(String(err));
    });

    await page.goto(baseUrl, { waitUntil: 'networkidle' });
    await page.waitForSelector('.brand');

    // Verify monochrome stone background matching mockup
    const bg = await page.evaluate(() => getComputedStyle(document.body).backgroundColor);
    if (bg !== 'rgb(250, 250, 249)') {
      throw new Error(`Unexpected body background: ${bg}`);
    }

    // 1. Click "+ New draft"
    await page.getByRole('button', { name: '+ New draft' }).click();
    await page.waitForFunction(() => {
      const el = document.querySelector('[data-testid="header-branch"]');
      return el && el.textContent && el.textContent.startsWith('studio/untitled');
    });

    // 2. Fill frontmatter fields
    await page.getByLabel('Title').fill('E2E Studio Post');
    await page.getByLabel('Date').fill('2026-11-04');
    await page.getByLabel('Topic').selectOption('ai-agents');
    await page.getByLabel('Summary').fill('End-to-end Playwright test of Blog Studio.');
    await page.getByLabel('Project').fill('nacfson_blog');
    await page.getByLabel('Tags').fill('e2e, studio');

    // Verify auto-derived slug
    const slugVal = await page.getByLabel(/Slug/).inputValue();
    if (slugVal !== 'e2e-studio-post') {
      throw new Error(`Expected auto slug 'e2e-studio-post', got '${slugVal}'`);
    }

    // 3. Insert a custom block template into CodeMirror editor and verify live preview
    await page.getByRole('button', { name: 'benchmarkTable' }).click();
    await page.waitForFunction(() => {
      const preview = document.querySelector('[data-testid="studio-preview"]');
      return preview && preview.textContent && preview.textContent.includes('E2E Studio Post');
    });

    // Take screenshot of the studio layout
    await page.screenshot({ path: screenshotPath, fullPage: false });

    // 4. Click Save
    const saveBtn = page.getByRole('button', { name: /Save/ });
    await saveBtn.click();
    await page.waitForFunction(() => {
      const st = document.querySelector('[data-testid="header-status"]');
      return st && st.textContent && st.textContent.includes('Saved locally');
    });

    // 5. Click Publish… and confirm Push to main
    await page.getByRole('button', { name: 'Publish…' }).click();
    await page.waitForSelector('[data-testid="publish-diff"]');

    const pushBtn = page.getByRole('button', { name: 'Push to main' });
    await pushBtn.click();

    await page.waitForFunction(() => {
      const dlg = document.querySelector('.dialog');
      return dlg && dlg.textContent && dlg.textContent.includes('Published ');
    });

    await page.getByRole('button', { name: 'Done' }).click();

    if (consoleErrors.length > 0) {
      throw new Error(`Console errors: ${consoleErrors.join('; ')}`);
    }
    if (pageErrors.length > 0) {
      throw new Error(`Page errors: ${pageErrors.join('; ')}`);
    }

    console.log(JSON.stringify({ ok: true, screenshot: screenshotPath }));
  } finally {
    await browser.close();
  }
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
