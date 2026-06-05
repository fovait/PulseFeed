/** @type {import('tailwindcss').Config} */
export default {
  content: ["./index.html", "./src/**/*.{ts,tsx}"],
  theme: {
    extend: {
      colors: {
        pulse: {
          black: "#050505",
          panel: "rgba(17, 17, 20, 0.78)",
          line: "rgba(255, 255, 255, 0.12)",
          cyan: "#22d3ee",
          red: "#fb3c68",
        },
      },
      boxShadow: {
        glow: "0 18px 80px rgba(34, 211, 238, 0.16)",
      },
    },
  },
  plugins: [],
};
