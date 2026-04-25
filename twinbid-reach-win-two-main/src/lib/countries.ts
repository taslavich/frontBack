// ISO codes used in mock dimension data, mapped to EN + RU names.
export const COUNTRY_NAMES: Record<string, { en: string; ru: string }> = {
  US: { en: "United States", ru: "США" },
  GB: { en: "United Kingdom", ru: "Великобритания" },
  DE: { en: "Germany", ru: "Германия" },
  FR: { en: "France", ru: "Франция" },
  BR: { en: "Brazil", ru: "Бразилия" },
  IN: { en: "India", ru: "Индия" },
  JP: { en: "Japan", ru: "Япония" },
  RU: { en: "Russia", ru: "Россия" },
  AU: { en: "Australia", ru: "Австралия" },
  CA: { en: "Canada", ru: "Канада" },
  ES: { en: "Spain", ru: "Испания" },
  IT: { en: "Italy", ru: "Италия" },
  KR: { en: "South Korea", ru: "Южная Корея" },
  TR: { en: "Turkey", ru: "Турция" },
  PL: { en: "Poland", ru: "Польша" },
};

export function formatCountryLabel(code: string, lang: "ru" | "en"): string {
  const entry = COUNTRY_NAMES[code];
  if (!entry) return code;
  const name = lang === "ru" ? entry.ru : entry.en;
  return `${name} (${code})`;
}
