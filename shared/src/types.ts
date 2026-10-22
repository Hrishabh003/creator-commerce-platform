export type Customer = { id: string; name: string; email: string };

export type Draft = {
  id: string;
  ticketId: string;
  body: string;
  status: 'pending' | 'approved' | 'rejected';
  sources: string[];
  createdAt: string;
};

export type Ticket = {
  id: string;
  subject: string;
  body: string;
  category: string;
  status: 'open' | 'replied' | 'closed';
  createdAt: string;
  customer: Customer | null;
  drafts: Draft[];
};

export type Order = {
  id: string;
  status: string;
  totalCents: number;
  createdAt: string;
  customer: Customer | null;
};

export type Product = { id: string; sku: string; name: string; priceCents: number; stock: number };

export type Campaign = {
  id: string;
  name: string;
  subject: string;
  status: 'draft' | 'scheduled' | 'sent';
  sentCount: number;
  scheduledFor: string | null;
};

export const formatMoney = (cents: number) => `$${(cents / 100).toFixed(2)}`;
