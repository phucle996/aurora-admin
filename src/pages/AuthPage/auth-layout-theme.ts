const lightBackground =
  "linear-gradient(150deg, #B39DDB 0%, #D1C4E9 20%, #F3E5F5 40%, #FCE4EC 60%, #FFCDD2 80%, #FFAB91 100%)";

const darkBackground = `
  radial-gradient(ellipse 70% 55% at 50% 50%, rgba(255, 20, 147, 0.15), transparent 50%),
  radial-gradient(ellipse 160% 130% at 10% 10%, rgba(0, 255, 255, 0.12), transparent 60%),
  radial-gradient(ellipse 160% 130% at 90% 90%, rgba(138, 43, 226, 0.18), transparent 65%),
  radial-gradient(ellipse 110% 50% at 80% 30%, rgba(255, 215, 0, 0.08), transparent 40%),
  #000000
`;

const leftDarkBackground =
  "linear-gradient(160deg, #070b1f 0%, #0a122c 45%, #05060e 100%)";
const leftLightBackground =
  "linear-gradient(150deg, #f2f4fb 0%, #f5f1f7 40%, #f6eaef 75%, #f7e6da 100%)";

export const authBackgrounds = {
  light: lightBackground,
  dark: darkBackground,
} as const;

export const leftBackgrounds = {
  light: leftLightBackground,
  dark: leftDarkBackground,
} as const;
