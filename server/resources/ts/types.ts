// Bentuk JSON yang disimpan Go. Satu sumber kebenaran untuk field builder, entry form, dan tree editor.
export type FieldType = 'text' | 'textarea' | 'markdown' | 'bard' | 'toggle' | 'date' | 'integer' | 'number' | 'select' | 'radio' | 'array' | 'assets' | 'taxonomy' | 'author' | 'link' | 'replicator' | 'section';
export interface Field { handle: string; type: FieldType; display: string; required?: boolean; instructions?: string; options?: Record<string, string>; fields?: Field[] }
export interface Blueprint { title?: string; template?: string; fields: Field[] }
export interface TreeItem { id?: string; type?: 'internal' | 'external' | 'page'; title?: string; url?: string; target?: string; page_id?: string; roles?: string[]; data?: Record<string, unknown>; children?: TreeItem[] }
