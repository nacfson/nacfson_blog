import type { Config } from 'tailwindcss';

const config: Config = {
  content: [
    './index.html',
    './src/**/*.{js,ts,jsx,tsx}',
    '../web/src/components/**/*.{js,ts,jsx,tsx}',
  ],
  theme: {
    extend: {
      colors: {
        canvas: '#fafaf9',
        card: '#ffffff',
        ink: '#0c0a09',
      },
    },
  },
  plugins: [],
};

export default config;
