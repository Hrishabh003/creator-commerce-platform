import { useCallback, useEffect, useState } from 'react';
import { createApi } from '../../shared/src/api';
import { formatMoney } from '../../shared/src/types';
import type { Campaign, Order, Product, Ticket } from '../../shared/src/types';

const api = createApi(import.meta.env.VITE_API_URL ?? 'http://localhost:8080/graphql');
type Tab = 'tickets' | 'orders' | 'products' | 'campaigns';

function useLoad<T>(fn: () => Promise<T>, initial: T) {
  const [data, setData] = useState<T>(initial);
  const [error, setError] = useState('');
  const reload = useCallback(() => {
    fn().then(setData).catch((e: Error) => setError(e.message));
  }, [fn]);
  useEffect(reload, [reload]);
  return { data, error, reload };
}

export default function App() {
  const [tab, setTab] = useState<Tab>('tickets');
  const tabs: [Tab, string][] = [['tickets', 'Support'], ['orders', 'Orders'], ['products', 'Products'], ['campaigns', 'Campaigns']];
  return (
    <div className="shell">
      <nav aria-label="Sections">
        <h1>Creator Commerce Ops</h1>
        {tabs.map(([id, label]) => (
          <button key={id} aria-current={tab === id ? 'page' : undefined} onClick={() => setTab(id)}>{label}</button>
        ))}
      </nav>
      <main>
        {tab === 'tickets' && <Tickets />}
        {tab === 'orders' && <Orders />}
        {tab === 'products' && <Products />}
        {tab === 'campaigns' && <Campaigns />}
      </main>
    </div>
  );
}

function Tickets() {
  const load = useCallback(() => api.tickets(), []);
  const { data: tickets, error, reload } = useLoad<Ticket[]>(load, []);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const selected = tickets.find((t) => t.id === selectedId) ?? tickets[0];
  const pending = selected?.drafts.find((d) => d.status === 'pending');
  const [text, setText] = useState('');
  const [busy, setBusy] = useState(false);
  const [form, setForm] = useState({ customerId: '1', subject: '', body: '' });

  useEffect(() => setText(pending?.body ?? ''), [pending?.id, pending?.body]);

  async function review(approve: boolean) {
    if (!pending) return;
    setBusy(true);
    try { await api.reviewDraft(pending.id, approve, approve ? text : undefined); reload(); }
    finally { setBusy(false); }
  }

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    try {
      const t = await api.createTicket(form.customerId, form.subject, form.body);
      setForm({ ...form, subject: '', body: '' });
      setSelectedId(t.id);
      reload();
    } finally { setBusy(false); }
  }

  return (
    <>
      <h2>Support</h2>
      {error && <p className="error" role="alert">{error}</p>}
      <div className="split">
        <div className="panel">
          {tickets.length === 0 && <p className="meta">No tickets yet. Create one below.</p>}
          {tickets.map((t) => (
            <button key={t.id} className="row" aria-selected={t.id === selected?.id} onClick={() => setSelectedId(t.id)}>
              <strong>{t.subject}</strong>
              <span className="tag">{t.category}</span>
              <div className="meta">{t.customer?.name} · {t.status}{t.drafts.some((d) => d.status === 'pending') && <span className="pending"> · draft to review</span>}</div>
            </button>
          ))}
          <form onSubmit={submit}>
            <label htmlFor="cid">Customer ID</label>
            <input id="cid" value={form.customerId} onChange={(e) => setForm({ ...form, customerId: e.target.value })} required />
            <label htmlFor="subj">Subject</label>
            <input id="subj" value={form.subject} onChange={(e) => setForm({ ...form, subject: e.target.value })} required />
            <label htmlFor="msg">Message</label>
            <textarea id="msg" style={{ minHeight: 80 }} value={form.body} onChange={(e) => setForm({ ...form, body: e.target.value })} required />
            <div className="actions"><button className="primary" disabled={busy}>Create ticket</button></div>
          </form>
        </div>
        {selected && (
          <div className="panel">
            <strong>{selected.subject}</strong>
            <p>{selected.body}</p>
            {pending ? (
              <>
                <label htmlFor="draft">Draft reply (edit before approving)</label>
                <textarea id="draft" value={text} onChange={(e) => setText(e.target.value)} />
                <p className="meta">Sources: {pending.sources.join(', ') || 'none found'}</p>
                <div className="actions">
                  <button className="primary" disabled={busy} onClick={() => review(true)}>Approve and send</button>
                  <button className="secondary" disabled={busy} onClick={() => review(false)}>Reject draft</button>
                </div>
              </>
            ) : <p className="meta">No draft waiting for review.</p>}
          </div>
        )}
      </div>
    </>
  );
}

function Orders() {
  const load = useCallback(() => api.orders(), []);
  const { data, error } = useLoad<Order[]>(load, []);
  return (
    <>
      <h2>Orders</h2>
      {error && <p className="error" role="alert">{error}</p>}
      <table>
        <thead><tr><th>Order</th><th>Customer</th><th>Status</th><th>Total</th></tr></thead>
        <tbody>{data.map((o) => (
          <tr key={o.id}><td>#{o.id}</td><td>{o.customer?.name}</td><td>{o.status}</td><td>{formatMoney(o.totalCents)}</td></tr>
        ))}</tbody>
      </table>
    </>
  );
}

function Products() {
  const load = useCallback(() => api.products(), []);
  const { data, error, reload } = useLoad<Product[]>(load, []);
  async function setStock(p: Product) {
    const v = window.prompt(`New stock for ${p.name}`, String(p.stock));
    if (v === null || Number.isNaN(Number(v)) || Number(v) < 0) return;
    await api.updateStock(p.id, Math.floor(Number(v)));
    reload();
  }
  return (
    <>
      <h2>Products</h2>
      {error && <p className="error" role="alert">{error}</p>}
      <table>
        <thead><tr><th>SKU</th><th>Name</th><th>Price</th><th>Stock</th><th /></tr></thead>
        <tbody>{data.map((p) => (
          <tr key={p.id}>
            <td>{p.sku}</td><td>{p.name}</td><td>{formatMoney(p.priceCents)}</td>
            <td className={p.stock === 0 ? 'error' : undefined}>{p.stock === 0 ? 'Out of stock' : p.stock}</td>
            <td><button className="secondary" onClick={() => setStock(p)}>Update stock</button></td>
          </tr>
        ))}</tbody>
      </table>
    </>
  );
}

function Campaigns() {
  const load = useCallback(() => api.campaigns(), []);
  const { data, error } = useLoad<Campaign[]>(load, []);
  return (
    <>
      <h2>Campaigns</h2>
      {error && <p className="error" role="alert">{error}</p>}
      <table>
        <thead><tr><th>Name</th><th>Subject</th><th>Status</th><th>Sent</th></tr></thead>
        <tbody>{data.map((c) => (
          <tr key={c.id}><td>{c.name}</td><td>{c.subject}</td><td>{c.status}</td><td>{c.sentCount}</td></tr>
        ))}</tbody>
      </table>
    </>
  );
}
