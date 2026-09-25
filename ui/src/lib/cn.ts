import { twMerge, type ClassNameValue } from 'tailwind-merge';

/** Joins class names, letting later Tailwind utilities override earlier ones. */
export function cn(...inputs: ClassNameValue[]): string {
  return twMerge(inputs);
}
