import type { Metadata } from 'next';
import './globals.css';
import Link from 'next/link';

export const metadata: Metadata = {
  title: 'nacfson // Cloud Native & Systems Engineering',
  description: 'Interactive frontier-research engineering blog and technical competency matrix.',
};

export default function RootLayout({
  children,
}: ReadageProps<{ children: React.ReactNode }>) {
  return (
    <html lang="en" className="bg-[#f8fafc] text-slate-900">
      <body className="min-h-screen flex flex-col bg-[#f8fafc]">
        {/* Top Navigation */}
        <header className="sticky top-0 z-40 border-b border-slate-200/80 bg-white/80 backdrop-blur-md">
          <div className="mx-auto flex h-16 max-w-7xl items-center justify-between px-4 sm:px-6 lg:px-8">
            <div className="flex items-center space-x-4">
              <Link
                href="/"
                className="flex items-center space-x-2.5 text-slate-900 hover:text-blue-600 transition-colors"
              >
                <div className="flex h-8 w-8 items-center justify-center rounded-lg bg-blue-600 text-white font-bold text-sm shadow-sm">
                  N
                </div>
                <div className="flex flex-col">
                  <span className="font-bold tracking-tight text-sm">nacfson</span>
                  <span className="text-[10px] text-slate-500 font-mono -mt-0.5">engineering blog</span>
                </div>
              </Link>
              <span className="hidden md:inline-flex items-center px-2 py-0.5 rounded-full text-[10px] font-medium bg-emerald-50 text-emerald-700 border border-emerald-200">
                <span className="w-1.5 h-1.5 mr-1 rounded-full bg-emerald-500 animate-pulse" />
                K3s Single-Pod
              </span>
            </div>

            <nav className="flex items-center space-x-6 text-xs font-medium text-slate-600">
              <Link href="/" className="hover:text-blue-600 transition-colors">
                Articles
              </Link>
              <a href="/#competencies" className="hover:text-blue-600 transition-colors">
                Competency Matrix
              </a>
              <a
                href="https://github.com/nacfson"
                target="_blank"
                rel="noopener noreferrer"
                className="inline-flex items-center gap-1 hover:text-slate-900 transition-colors"
              >
                <span>GitHub</span>
                <svg className="w-3.5 h-3.5 text-slate-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-4M14 4h6m0 0v6m0-6L10 14" />
                </svg>
              </a>
            </nav>
          </div>
        </header>

        {/* Main Content Area */}
        <main className="flex-1">{children}</main>

        {/* Footer */}
        <footer className="border-t border-slate-200 bg-white py-8 text-xs text-slate-500">
          <div className="mx-auto max-w-7xl px-4 sm:px-6 lg:px-8 flex flex-col sm:flex-row items-center justify-between gap-4">
            <div>
              <p className="font-medium text-slate-700">nacfson_blog &middot; proj-blog</p>
              <p className="text-[11px] text-slate-400 mt-0.5">
                Go 1.22 stdlib &middot; Next.js 14 static export &middot; FluxCD live tarball sync &middot; Cilium eBPF
              </p>
            </div>
            <div className="text-right text-[11px] text-slate-400">
              <span>Zero plaintext credentials &middot; Restricted PSS</span>
            </div>
          </div>
        </footer>
      </body>
    </html>
  );
}

// Helper type for Next.js layout props
type ReadageProps<T> = T;
