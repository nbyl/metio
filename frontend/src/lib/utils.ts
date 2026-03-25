import { clsx, type ClassValue } from 'clsx';
import { twMerge } from 'tailwind-merge';

/**
 * Utility function to merge Tailwind CSS classes with proper precedence.
 * Combines clsx for conditional classes and tailwind-merge for deduplication.
 *
 * @param inputs - Class values to merge (strings, objects, arrays)
 * @returns Merged class string
 *
 * @example
 * cn('px-2 py-1', 'px-4') // => 'py-1 px-4'
 * cn('btn', { 'btn-active': isActive }) // => 'btn btn-active' (if isActive)
 */
export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}
