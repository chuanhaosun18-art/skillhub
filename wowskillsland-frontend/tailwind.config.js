/** @type {import('tailwindcss').Config} */
export default {
  content: ["./index.html", "./src/**/*.{js,ts,jsx,tsx}"],
  theme: {
    extend: {
      fontFamily: {
        sans: ["Geist", "sans-serif"],
        display: ["Special Elite", "serif"],
      },
      colors: {
        skillx: {
          dark: "#0a0a0a",
          text: "#171a18",
          muted: "#626a64",
          prompt: "#234d38",
          accent: "#155eef",
        },
      },
    },
  },
  plugins: [],
};
