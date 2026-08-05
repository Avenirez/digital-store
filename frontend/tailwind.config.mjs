/** @type {import('tailwindcss').Config} */
export default {
  darkMode: 'class',
  content: ['./src/**/*.{astro,html,js,jsx,md,mdx,svelte,ts,tsx,vue}'],
  theme: {
    extend: {
      colors: {
        brand: {
          50:  '#fdfaf7',
          100: '#f7ede2',
          200: '#eddcd0',
          300: '#dfc5b3',
          400: '#cca590',
          500: '#b8856c', // Soft warm terracotta/brown accent
          600: '#a36d54',
          700: '#87533d',
          800: '#6f4332',
          900: '#5c392b',
          950: '#331d14',
        },
        surface: {
          50:  '#ffffff',
          100: '#fcf6f0',
          200: '#faf0e6', // #faf0e6 Linen background
          300: '#f3e3d3',
          400: '#e4cdb7',
          500: '#bca28b',
          600: '#8c735d',
          700: '#5c4a3b',
          800: '#3d3025',
          850: '#2b2119',
          900: '#1c1510',
          950: '#120d09',
        },
      },
      fontFamily: {
        sans: ['Epilogue', 'system-ui', '-apple-system', 'sans-serif'],
        mono: ['JetBrains Mono', 'Fira Code', 'monospace'],
      },
      animation: {
        'fade-in': 'fadeIn 0.5s ease-out',
        'slide-up': 'slideUp 0.5s ease-out',
        'pulse-soft': 'pulseSoft 2s ease-in-out infinite',
        'shimmer': 'shimmer 2s linear infinite',
      },
      keyframes: {
        fadeIn: {
          '0%': { opacity: '0' },
          '100%': { opacity: '1' },
        },
        slideUp: {
          '0%': { opacity: '0', transform: 'translateY(20px)' },
          '100%': { opacity: '1', transform: 'translateY(0)' },
        },
        pulseSoft: {
          '0%, 100%': { opacity: '1' },
          '50%': { opacity: '0.7' },
        },
        shimmer: {
          '0%': { backgroundPosition: '-200% 0' },
          '100%': { backgroundPosition: '200% 0' },
        },
      },
      backgroundImage: {
        'gradient-radial': 'radial-gradient(var(--tw-gradient-stops))',
        'grid-pattern': 'linear-gradient(to right, rgba(99, 102, 241, 0.05) 1px, transparent 1px), linear-gradient(to bottom, rgba(99, 102, 241, 0.05) 1px, transparent 1px)',
      },
      backgroundSize: {
        'grid': '40px 40px',
      },
    },
  },
  plugins: [],
};
