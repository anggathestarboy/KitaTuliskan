// Padanan Alpine.data('bardEditor') di resources/js/app.js + blok bard di entry-field.blade.php: editor Tiptap dengan toolbar
// sesuai `toolbar` field, gambar/video/audio/file dari asset picker, menu Replace/Delete saat media diklik.
import { Editor, Node, mergeAttributes } from '@tiptap/core';
import Image from '@tiptap/extension-image';
import StarterKit from '@tiptap/starter-kit';
import { useEffect, useRef, useState } from 'react';
import { type Asset } from '../media/api';
import { Icon, cn } from '../shared/ui';
import { type Field } from './api';

// Video/audio/file tidak punya node bawaan Tiptap; tanpa ini tag-nya dibuang saat parse berikutnya.
const media = (name: string, tag: string) => Node.create({
  name, group: 'block', atom: true, draggable: true,
  addAttributes: () => ({ src: { default: null } }),
  parseHTML: () => [{ tag: `${tag}[src]` }],
  renderHTML: ({ HTMLAttributes }) => [tag, mergeAttributes(HTMLAttributes, { controls: 'controls' })],
});
const Video = media('video', 'video');
const Audio = media('audio', 'audio');
const FileAttachment = Node.create({
  name: 'fileAttachment', group: 'block', atom: true, draggable: true,
  addAttributes: () => ({ src: { default: null }, filename: { default: null } }),
  parseHTML: () => [{ tag: 'a[data-file-attachment]' }],
  renderHTML: ({ HTMLAttributes, node }) => ['a', mergeAttributes(HTMLAttributes, { href: node.attrs.src, download: node.attrs.filename, 'data-file-attachment': '', class: 'bard-file' }), `\u{1F4CE} ${node.attrs.filename || node.attrs.src}`],
});

// Node skema menurut mime type asset.
const nodeFor = (a: Asset) => {
  if (a.mime_type.startsWith('image/')) return { type: 'image', attrs: { src: a.url } };
  if (a.mime_type.startsWith('video/')) return { type: 'video', attrs: { src: a.url } };
  if (a.mime_type.startsWith('audio/')) return { type: 'audio', attrs: { src: a.url } };
  return { type: 'fileAttachment', attrs: { src: a.url, filename: a.title } };
};

const btn = 'rounded p-1.5 text-zinc-500 hover:bg-zinc-100 data-[active]:bg-zinc-200 data-[active]:text-zinc-900 dark:text-zinc-300 dark:hover:bg-white/10 dark:data-[active]:bg-white/20 dark:data-[active]:text-white';
const wide = 'rounded px-2 py-1 text-xs font-semibold text-zinc-500 hover:bg-zinc-100 data-[active]:bg-zinc-200 data-[active]:text-zinc-900 dark:text-zinc-300 dark:hover:bg-white/10 dark:data-[active]:bg-white/20 dark:data-[active]:text-white';
const sep = <div className="mx-1 h-5 w-px bg-zinc-200 dark:bg-white/10" />;

// Gaya isi dokumen; dipakai juga oleh tampilan read-only field non-localizable.
export const PROSE = 'leading-relaxed [&_h1]:mb-2 [&_h1]:mt-5 [&_h1]:text-xl [&_h1]:font-bold [&_h2]:mb-2 [&_h2]:mt-5 [&_h2]:text-lg [&_h2]:font-semibold [&_h3]:mb-2 [&_h3]:mt-5 [&_h3]:text-base [&_h3]:font-semibold [&_h4]:mb-2 [&_h4]:mt-4 [&_h4]:text-base [&_h4]:font-semibold [&_h5]:mb-2 [&_h5]:mt-4 [&_h5]:text-sm [&_h5]:font-semibold [&_h6]:mb-2 [&_h6]:mt-4 [&_h6]:text-sm [&_h6]:font-semibold [&_p]:my-3 [&_ul]:my-3 [&_ul]:list-disc [&_ul]:pl-5 [&_ol]:my-3 [&_ol]:list-decimal [&_ol]:pl-5 [&_li]:my-1 [&_li_p]:my-0 [&_li_ul]:my-1 [&_li_ol]:my-1 [&_blockquote]:my-3 [&_blockquote]:border-l-2 [&_blockquote]:border-zinc-300 [&_blockquote]:pl-3 [&_blockquote]:italic dark:[&_blockquote]:border-zinc-600 [&_hr]:my-4 [&_a]:text-blue-600 [&_a]:underline dark:[&_a]:text-blue-400 [&_img]:my-3 [&_img]:max-w-full [&_img]:max-h-[350px] [&_img]:w-full [&_img]:bg-gray-200 [&_img]:object-contain [&_img]:rounded-lg [&_:first-child]:mt-0 [&_:last-child]:mb-0';
const EDITOR = `relative max-w-none px-4 py-3 text-sm text-zinc-800 dark:text-zinc-200 [&_.ProseMirror]:min-h-32 [&_.ProseMirror]:outline-none ${PROSE} [&_img]:cursor-pointer [&_img.ProseMirror-selectednode]:ring-2 [&_img.ProseMirror-selectednode]:ring-blue-500 [&_video]:my-3 [&_video]:max-w-full [&_video]:w-full [&_video]:cursor-pointer [&_video]:rounded-lg [&_video.ProseMirror-selectednode]:ring-2 [&_video.ProseMirror-selectednode]:ring-blue-500 [&_audio]:my-3 [&_audio]:w-full [&_audio]:cursor-pointer [&_audio.ProseMirror-selectednode]:ring-2 [&_audio.ProseMirror-selectednode]:ring-blue-500 [&_audio.ProseMirror-selectednode]:rounded-lg [&_.bard-file]:my-3 [&_.bard-file]:inline-flex [&_.bard-file]:cursor-pointer [&_.bard-file]:rounded-lg [&_.bard-file]:border [&_.bard-file]:border-zinc-200 [&_.bard-file]:px-3 [&_.bard-file]:py-1.5 [&_.bard-file]:no-underline dark:[&_.bard-file]:border-white/10 [&_.bard-file.ProseMirror-selectednode]:ring-2 [&_.bard-file.ProseMirror-selectednode]:ring-blue-500`;

export function Bard({ f, value, onChange, container, pickMedia }: {
  f: Field; value: string; onChange: (html: string) => void; container: string;
  pickMedia: (container: string, single: boolean, ok: (assets: Asset[]) => void) => void;
}) {
  const host = useRef<HTMLDivElement>(null);
  const editor = useRef<Editor | null>(null);
  const [, tick] = useState(0);
  const [menu, setMenu] = useState<{ pos: number; top: number; left: number } | null>(null);
  const change = useRef(onChange);
  change.current = onChange;

  // Editor dibuat sekali (isi awal saja); perubahan dikirim ke atas, sama seperti wire:ignore di source.
  useEffect(() => {
    const e = new Editor({
      element: host.current!,
      // Link & Underline sudah ada di StarterKit Tiptap v3; mengimpornya lagi mendaftarkan extension dua kali.
      extensions: [StarterKit.configure({ link: { openOnClick: false } }), Image, Video, Audio, FileAttachment],
      content: value || '',
      onUpdate: ({ editor: ed }) => { setMenu(null); change.current(ed.isEmpty ? '' : ed.getHTML()); },
      onTransaction: () => tick((n) => n + 1),
    });
    editor.current = e;
    return () => { e.destroy(); editor.current = null; };
  }, []);

  const ed = editor.current;
  const shows = (k: string) => !f.toolbar?.length || f.toolbar.includes(k);
  const active = (name: string, attrs?: Record<string, unknown>) => (ed?.isActive(name, attrs) ? true : undefined);
  const run = (fn: (c: ReturnType<Editor['chain']>) => ReturnType<Editor['chain']>) => fn(editor.current!.chain().focus()).run();
  const B = (k: string, icon: string, name: string, cmd: (c: ReturnType<Editor['chain']>) => ReturnType<Editor['chain']>) =>
    shows(k) && <button key={k} type="button" onMouseDown={(e) => e.preventDefault()} onClick={() => run(cmd)} data-active={active(name)} className={btn}><Icon name={icon} /></button>;

  const setLink = () => {
    const url = window.prompt('URL');
    if (url === null) return;
    run((c) => (url === '' ? c.unsetLink() : c.setLink({ href: url })));
  };

  const onClick = (e: React.MouseEvent) => {
    const el = (e.target as HTMLElement).closest('img, video, audio, [data-file-attachment]');
    if (!el) return setMenu(null);
    e.preventDefault(); // tautan file tidak boleh membuka unduhan saat mengedit
    const pos = editor.current!.view.posAtDOM(el, 0);
    const r = el.getBoundingClientRect(), box = host.current!.getBoundingClientRect();
    editor.current!.chain().setNodeSelection(pos).run();
    setMenu({ pos, top: r.bottom - box.top + 4, left: r.left - box.left });
  };

  const insert = () => pickMedia(container, false, (assets) => run((c) => c.insertContent(assets.map(nodeFor))));
  const replace = () => {
    const pos = menu!.pos; setMenu(null);
    pickMedia(container, true, (assets) => editor.current!.chain().focus().deleteRange({ from: pos, to: pos + 1 }).insertContentAt(pos, nodeFor(assets[0])).run());
  };

  return (
    <div className="rounded-lg border border-zinc-200 shadow-xs dark:border-white/10">
      <div className="flex flex-wrap items-center gap-1 border-b border-zinc-200 p-1.5 dark:border-white/10">
        {B('bold', 'bold', 'bold', (c) => c.toggleBold())}
        {B('italic', 'italic', 'italic', (c) => c.toggleItalic())}
        {B('underline', 'underline', 'underline', (c) => c.toggleUnderline())}
        {B('strike', 'strikethrough', 'strike', (c) => c.toggleStrike())}
        {[1, 2, 3, 4, 5, 6].some((l) => shows(`h${l}`)) && sep}
        {([1, 2, 3, 4, 5, 6] as const).map((l) => shows(`h${l}`) && <button key={l} type="button" onMouseDown={(e) => e.preventDefault()} onClick={() => run((c) => c.toggleHeading({ level: l }))} data-active={active('heading', { level: l })} className={wide}>H{l}</button>)}
        {(shows('bulletlist') || shows('orderedlist') || shows('blockquote') || shows('link') || shows('image')) && sep}
        {B('bulletlist', 'list-bullet', 'bulletList', (c) => c.toggleBulletList())}
        {B('orderedlist', 'numbered-list', 'orderedList', (c) => c.toggleOrderedList())}
        {shows('blockquote') && <button type="button" onMouseDown={(e) => e.preventDefault()} onClick={() => run((c) => c.toggleBlockquote())} data-active={active('blockquote')} className={cn(btn.replace('p-1.5', 'px-2 py-1 text-sm'))}>"</button>}
        {shows('link') && <button type="button" onMouseDown={(e) => e.preventDefault()} onClick={setLink} data-active={active('link')} className={btn}><Icon name="link" /></button>}
        {shows('image') && container && <button type="button" onMouseDown={(e) => e.preventDefault()} onClick={insert} className={btn}><Icon name="photo" /></button>}
        {sep}
        <button type="button" onMouseDown={(e) => e.preventDefault()} onClick={() => run((c) => c.undo())} className="rounded p-1.5 text-zinc-500 hover:bg-zinc-100 dark:text-zinc-300 dark:hover:bg-white/10"><Icon name="arrow-uturn-left" /></button>
        <button type="button" onMouseDown={(e) => e.preventDefault()} onClick={() => run((c) => c.redo())} className="rounded p-1.5 text-zinc-500 hover:bg-zinc-100 dark:text-zinc-300 dark:hover:bg-white/10"><Icon name="arrow-uturn-right" /></button>
      </div>
      <div className={EDITOR} onClick={onClick}>
        <div ref={host} />
        {menu && (
          <div style={{ top: menu.top, left: menu.left }} className="absolute z-10 flex items-center gap-1 rounded-lg border border-zinc-200 bg-white p-1 text-xs shadow-md dark:border-white/10 dark:bg-zinc-800">
            {container && <button type="button" onClick={replace} className="rounded px-2 py-1 font-medium text-zinc-600 hover:bg-zinc-100 dark:text-zinc-300 dark:hover:bg-white/10">Replace</button>}
            <button type="button" onClick={() => { editor.current!.chain().focus().deleteRange({ from: menu.pos, to: menu.pos + 1 }).run(); setMenu(null); }} className="rounded px-2 py-1 font-medium text-red-600 hover:bg-red-50 dark:text-red-400 dark:hover:bg-red-500/10">Delete</button>
          </div>
        )}
      </div>
    </div>
  );
}
