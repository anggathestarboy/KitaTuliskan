// DIGENERATE oleh plugins/cms-types.mjs dari blueprint CMS. Jangan diedit.
interface CmsAsset { url: string; alt?: string; width?: number; height?: number; loading?: string; title?: string; mime_type?: string; [k: string]: any }
type CmsEntry<T> = import('./cms').Entry & T & { url: string; data: T };
declare namespace Cms {
  type Navigation = { "header": { "menu_loop"?: Array<{ _enabled?: boolean; "menu_label"?: string; "menu_url"?: string; }>; }; "footer": { "menu_loop"?: Array<{ _enabled?: boolean; "menu_label"?: string; "menu_url"?: string; }>; }; "sidebar-admin": any; };
  type Global = { "site_settings": { "site_name"?: string; "tagline"?: string; }; };
  type Collection = { "pages": CmsEntry<{ "heading"?: string; "subheading"?: string; "content"?: string; "title"?: string; "content"?: string; }>[]; };
  type Taxonomy = {  };
}
