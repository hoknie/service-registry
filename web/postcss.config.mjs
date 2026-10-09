// Tailwind CSS v4 through PostCSS (ADR-0030); the build inlines everything into the export.
const config = { plugins: { "@tailwindcss/postcss": {} } };

export default config;
