import { defineConfig } from 'astro/config';
import tailwind from '@astrojs/tailwind';

// https://astro.build/config
export default defineConfig({
  integrations: [tailwind()],
  devToolbar: { enabled: false },
  output: 'static',
  site: 'https://store.example.com',
  vite: {
    define: {
      'import.meta.env.PUBLIC_API_URL': JSON.stringify(
        process.env.PUBLIC_API_URL || 'http://localhost:8080'
      ),
    },
  },
});
