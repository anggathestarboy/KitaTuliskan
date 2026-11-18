// Padanan site-settings::partials.schema-block (via HasSchemaBlocks): tab "Schema" entry, blok schema.org JSON-LD per entry.
import { area } from '../navigation/Fields';
import { Button, Input, Select, Text } from '../shared/ui';
import { FL } from './Field';

export interface Block {
  type: string; name: string; alternate_name: string; description: string; logo: string; same_as: string[]; email: string; telephone: string; contact_type: string;
  address_street: string; address_locality: string; address_region: string; address_postal: string; address_country: string; service_type: string; area_served: string;
}
export const blankBlock = (): Block => ({
  type: 'Organization', name: '', alternate_name: '', description: '', logo: '', same_as: [], email: '', telephone: '', contact_type: '',
  address_street: '', address_locality: '', address_region: '', address_postal: '', address_country: '', service_type: '', area_served: '',
});

export function SchemaTab({ blocks, setBlocks, types, errors, canPick, pickLogo }: {
  blocks: Block[]; setBlocks: (b: Block[]) => void; types: string[]; errors: Record<string, string>; canPick: boolean; pickLogo: (i: number) => void;
}) {
  const upd = (i: number, p: Partial<Block>) => setBlocks(blocks.map((b, x) => (x === i ? { ...b, ...p } : b)));
  return (
    <div className="space-y-6">
      <Text>Schema.org JSON-LD blocks for this entry specifically — rendered alongside the site-wide blocks from Site Settings.</Text>
      <div className="space-y-4">
        {blocks.length === 0 && <Text>No schema blocks yet.</Text>}
        {blocks.map((b, i) => {
          const e = (k: string) => errors[`schemaBlocks.${i}.${k}`];
          const t = (k: keyof Block, label: string, ph?: string, type?: string) => (
            <FL label={label} err={e(k)}><Input type={type} placeholder={ph} value={b[k] as string} onChange={(x) => upd(i, { [k]: x.target.value })} /></FL>
          );
          return (
            <div key={i} className="space-y-4 rounded-lg border border-zinc-200 p-4 dark:border-white/10">
              <div className="flex items-center justify-between gap-4">
                <div className="w-full max-w-xs">
                  <FL label="Schema type" err={e('type')}>
                    <Select value={b.type} onChange={(x) => upd(i, { type: x.target.value })}>{types.map((x) => <option key={x} value={x}>{x}</option>)}</Select>
                  </FL>
                </div>
                <Button variant="danger" onClick={() => setBlocks(blocks.filter((_, x) => x !== i))}>Remove</Button>
              </div>
              {t('name', 'Name', 'Acme Inc.')}
              {b.type === 'Organization' && t('alternate_name', 'Alternate name', 'Acme')}
              <FL label="Description" err={e('description')}>
                <textarea rows={3} className={area} value={b.description} onChange={(x) => upd(i, { description: x.target.value })} />
              </FL>
              {['Organization', 'ContactPage', 'AboutPage'].includes(b.type) && (
                <FL label="Logo / image" err={e('logo')}>
                  <div className="flex items-center gap-3">
                    {b.logo
                      ? <div className="flex items-center gap-3"><img src={b.logo} className="size-16 rounded border border-zinc-200 object-contain dark:border-white/10" alt="" /><Button variant="danger" onClick={() => upd(i, { logo: '' })}>Remove</Button></div>
                      : <Button disabled={!canPick} onClick={() => pickLogo(i)}>Choose logo</Button>}
                  </div>
                </FL>
              )}
              {b.type === 'Organization' && (
                <div className="space-y-4">
                  <FL label="Social profiles (sameAs)" err={e('same_as')}>
                    <div className="space-y-2">
                      {b.same_as.map((u, j) => (
                        <div key={j} className="flex gap-2">
                          <div className="flex-1"><Input placeholder="https://twitter.com/acme" value={u} onChange={(x) => upd(i, { same_as: b.same_as.map((v, k) => (k === j ? x.target.value : v)) })} /></div>
                          <Button variant="ghost" icon="x-mark" aria-label="Remove" className="size-10!" onClick={() => upd(i, { same_as: b.same_as.filter((_, k) => k !== j) })} />
                        </div>
                      ))}
                    </div>
                    <div className="mt-2"><Button onClick={() => upd(i, { same_as: [...b.same_as, ''] })}>Add profile</Button></div>
                  </FL>
                  <div className="grid grid-cols-2 gap-4">{t('email', 'Email', 'hello@acme.com', 'email')}{t('telephone', 'Telephone', '+62 812 3456 7890')}</div>
                  {t('contact_type', 'Contact type', 'customer service')}
                  <div className="space-y-4 border-t border-zinc-200 pt-4 dark:border-white/10">
                    <Text>Address</Text>
                    {t('address_street', 'Street address')}
                    <div className="grid grid-cols-2 gap-4">{t('address_locality', 'City')}{t('address_region', 'Region/Province')}</div>
                    <div className="grid grid-cols-2 gap-4">{t('address_postal', 'Postal code')}{t('address_country', 'Country', 'ID')}</div>
                  </div>
                </div>
              )}
              {b.type === 'Service' && <div className="space-y-4">{t('service_type', 'Service type', 'Web Design')}{t('area_served', 'Area served', 'Indonesia')}</div>}
            </div>
          );
        })}
      </div>
      <Button onClick={() => setBlocks([...blocks, blankBlock()])}>Add schema</Button>
    </div>
  );
}
