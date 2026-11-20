// flux:field + flux:label (+ bintang wajib), deskripsi, dan pesan error: pembungkus untuk input form entry yang bukan field blueprint.
import { type ReactNode } from 'react';
import { desc, errCls } from '../navigation/Fields';

export const FL = ({ label, required, description, err, children }: { label: string; required?: boolean; description?: string; err?: string; children: ReactNode }) => (
  <div>
    <label className="mb-3 inline-flex items-center text-sm font-medium text-zinc-800 dark:text-white">{label}{required && <>&nbsp;<span className="text-red-500">*</span></>}</label>
    {children}
    {description && <p className={desc}>{description}</p>}
    {err && <p className={errCls}>{err}</p>}
  </div>
);
