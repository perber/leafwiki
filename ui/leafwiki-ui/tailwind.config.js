/** @type {import('tailwindcss').Config} */
export default {
  content: [
    "./index.html",
    "./src/**/*.{js,ts,jsx,tsx}",
  ],
  theme: {
    extend: {
      colors: {
        wiki : {
          bg: '#f9fafb', // light background
          surface: '#ffffff',
          border: '#e5e7eb',
          text: '#111827',
          accent: '#4ade80', // soft green
          muted: '#6b7280',
        },
      },
      fontFamily: {
        sans: ['InterVariable', 'system-ui', 'sans-serif'],
      },
    },
  },
  plugins: [],
}

