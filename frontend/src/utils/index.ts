export function cleanLLMOutput(text: string): string {
  if (!text) return '';
  return text
    .replace(/```[\s\S]*?```/g, (match) => match) // preserve code blocks
    .replace(/\n{3,}/g, '\n\n') // collapse excess newlines
    .trim();
}

export function cleanLLMOutputLight(text: string): string {
  if (!text) return '';
  return text.replace(/\n{3,}/g, '\n\n').trim();
}
