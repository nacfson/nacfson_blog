import type { Metadata } from 'next';
import './globals.css';
import Link from 'next/link';

export const metadata: Metadata = {
  title: 'Make Better World | nacfson',
  description: 'nacfson\'s engineering blog on cloud-native platforms, zero-trust security, AI agents, and open source.',
};

export default function RootLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <html lang="en" className="bg-stone-50 text-stone-950">
      <body className="min-h-screen flex flex-col bg-stone-50">
        {/* Top Navigation */}
        <header className="sticky top-0 z-40 border-b border-stone-200 bg-white/90 backdrop-blur-md">
          <div className="mx-auto flex h-16 max-w-7xl items-center justify-between px-4 sm:px-6 lg:px-8">
            <Link href="/" className="flex items-center space-x-2.5 text-stone-950">
              <div className="flex h-8 w-8 items-center justify-center rounded bg-stone-950 text-stone-50 font-serif font-bold text-sm">
                N
              </div>
              <div className="flex flex-col">
                <span className="font-bold tracking-tight text-sm">nacfson</span>
                <span className="text-[10px] text-stone-500 font-mono -mt-0.5">engineering blog</span>
              </div>
            </Link>

            <nav className="flex items-center space-x-6 text-xs font-medium text-stone-600">
              <Link href="/" className="hover:text-stone-950 hover:underline underline-offset-4">
                Articles
              </Link>
              <a
                href="https://github.com/nacfson"
                target="_blank"
                rel="noopener noreferrer"
                className="inline-flex items-center gap-1 hover:text-stone-950 hover:underline underline-offset-4"
              >
                <span>GitHub</span>
                <span aria-hidden="true">↗</span>
              </a>
            </nav>
          </div>
        </header>

        {/* Sub-navigation */}
        <div className="border-b border-stone-200 bg-white">
          <div className="mx-auto flex h-11 max-w-7xl items-center gap-7 px-4 sm:px-6 lg:px-8 text-xs">
            <span className="border-r border-stone-200 pr-5 font-serif text-[13px] font-bold text-stone-950">Make Better World</span>
            <Link href="/" className="flex h-11 items-center border-b-2 border-stone-950 font-semibold text-stone-950">
              Articles
            </Link>
          </div>
        </div>

        {/* Main Content Area */}
        <main className="flex-1">{children}</main>

        {/* Footer */}
        <footer className="border-t border-stone-200 bg-white pt-12 pb-7 text-xs">
          <div className="mx-auto max-w-7xl px-4 sm:px-6 lg:px-8">
            <div className="grid grid-cols-2 gap-8 md:grid-cols-[2fr_1fr_1fr]">
              <div className="col-span-2 md:col-span-1">
                <p className="font-serif text-sm font-bold text-stone-950">Make Better World</p>
                <p className="mt-2 max-w-sm leading-relaxed text-stone-500">
                  nacfson&apos;s engineering log on cloud-native platforms, zero-trust security, AI agents, and open source.
                </p>
              </div>
              <div>
                <p className="mb-3 font-semibold text-stone-950">Topics</p>
                <ul className="space-y-2 text-stone-500">
                  <li><a href="/?topic=cloud-native" className="hover:text-stone-950 hover:underline underline-offset-4">Cloud-Native</a></li>
                  <li><a href="/?topic=zero-trust" className="hover:text-stone-950 hover:underline underline-offset-4">Zero-Trust</a></li>
                  <li><a href="/?topic=ai-agents" className="hover:text-stone-950 hover:underline underline-offset-4">AI Agents</a></li>
                  <li><a href="/?topic=open-source" className="hover:text-stone-950 hover:underline underline-offset-4">Open Source</a></li>
                </ul>
              </div>
              <div>
                <p className="mb-3 font-semibold text-stone-950">Elsewhere</p>
                <ul className="space-y-2 text-stone-500">
                  <li>
                    <a
                      href="https://github.com/nacfson"
                      target="_blank"
                      rel="noopener noreferrer"
                      className="hover:text-stone-950 hover:underline underline-offset-4"
                    >
                      GitHub
                    </a>
                  </li>
                </ul>
              </div>
            </div>
            <div className="mt-10 flex flex-col gap-2 border-t border-stone-100 pt-4 text-[11px] text-stone-400 sm:flex-row sm:justify-between">
              <span>&copy; 2026 nacfson</span>
              <span>Built with Go + Next.js</span>
            </div>
          </div>
        </footer>
      </body>
    </html>
  );
}

// Helper type for Next.js layout props
type ReadageProps<T> = T;
