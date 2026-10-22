import type { Campaign, Draft, Order, Product, Ticket } from './types';

const TICKET_FIELDS = `
  id subject body category status createdAt
  customer { id name email }
  drafts { id ticketId body status sources createdAt }`;

/** One typed GraphQL client, used by both the React web app and the Expo app. */
export function createApi(url: string) {
  async function gql<T>(query: string, variables?: Record<string, unknown>): Promise<T> {
    const res = await fetch(url, {
      method: 'POST',
      headers: { 'content-type': 'application/json' },
      body: JSON.stringify({ query, variables }),
    });
    if (!res.ok) throw new Error(`Request failed (${res.status})`);
    const json = await res.json();
    if (json.errors?.length) throw new Error(json.errors[0].message);
    return json.data as T;
  }

  return {
    tickets: async (status?: string) =>
      (await gql<{ tickets: Ticket[] }>(
        `query($status: String) { tickets(status: $status) { ${TICKET_FIELDS} } }`,
        { status },
      )).tickets,

    createTicket: async (customerId: string, subject: string, body: string) =>
      (await gql<{ createTicket: Ticket }>(
        `mutation($customerId: ID!, $subject: String!, $body: String!) {
           createTicket(customerId: $customerId, subject: $subject, body: $body) { ${TICKET_FIELDS} } }`,
        { customerId, subject, body },
      )).createTicket,

    reviewDraft: async (id: string, approve: boolean, body?: string) =>
      (await gql<{ reviewDraft: Draft }>(
        `mutation($id: ID!, $approve: Boolean!, $body: String) {
           reviewDraft(id: $id, approve: $approve, body: $body) { id status body } }`,
        { id, approve, body },
      )).reviewDraft,

    orders: async (status?: string) =>
      (await gql<{ orders: Order[] }>(
        `query($status: String) { orders(status: $status) { id status totalCents createdAt customer { id name email } } }`,
        { status },
      )).orders,

    products: async () =>
      (await gql<{ products: Product[] }>(`{ products { id sku name priceCents stock } }`)).products,

    updateStock: async (productId: string, stock: number) =>
      (await gql<{ updateStock: Product }>(
        `mutation($productId: ID!, $stock: Int!) { updateStock(productId: $productId, stock: $stock) { id stock } }`,
        { productId, stock },
      )).updateStock,

    campaigns: async () =>
      (await gql<{ campaigns: Campaign[] }>(
        `{ campaigns { id name subject status sentCount scheduledFor } }`,
      )).campaigns,
  };
}
