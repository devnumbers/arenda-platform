// Typographic post-processing: glue short prepositions / conjunctions / particles
// to the following word with a non-breaking space ( ) so they never dangle
// at the end of a line. Only spaces are replaced — text, punctuation and order
// are left untouched.

// Longer variants must come first so the alternation matches them greedily
// (e.g. "во" before "в", "изо" before "из").
const WORDS = [
  "обо",
  "изо",
  "во",
  "со",
  "ко",
  "об",
  "от",
  "до",
  "по",
  "из",
  "за",
  "под",
  "над",
  "при",
  "без",
  "для",
  "или",
  "либо",
  "да",
  "но",
  "не",
  "ни",
  "в",
  "на",
  "с",
  "к",
  "о",
  "у",
  "и",
  "а",
];

const RE = new RegExp(`(?<=^|[\\s(«"„.,:;—–-])(${WORDS.join("|")}) `, "giu");

export function typo(text: string): string {
  // Replace only the trailing regular space of each match with
  return text.replace(RE, (m) => m.slice(0, -1) + " ");
}
