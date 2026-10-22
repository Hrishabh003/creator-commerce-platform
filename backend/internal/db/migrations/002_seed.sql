INSERT INTO products (sku, name, price_cents, stock) VALUES
  ('STK-DIE-3', 'Die-cut sticker, 3 in', 299, 480),
  ('STK-HOL-2', 'Holographic sticker, 2 in', 349, 120),
  ('MAG-4', 'Custom magnet, 4 in', 599, 60),
  ('PIN-1', 'Enamel pin, 1 in', 899, 0);

INSERT INTO customers (name, email) VALUES
  ('Maya Chen', 'maya@example.com'),
  ('Tom Alvarez', 'tom@example.com'),
  ('Priya Nair', 'priya@example.com'),
  ('Sam Okafor', 'sam@example.com');

INSERT INTO orders (customer_id, status, total_cents, created_at) VALUES
  (1, 'delivered', 2990, now() - interval '9 days'),
  (2, 'shipped',   3490, now() - interval '3 days'),
  (3, 'paid',      5990, now() - interval '1 day'),
  (1, 'pending',    899, now() - interval '2 hours'),
  (4, 'refunded',  2990, now() - interval '14 days');

INSERT INTO order_items (order_id, product_id, quantity, unit_price_cents) VALUES
  (1, 1, 10, 299), (2, 2, 10, 349), (3, 3, 10, 599), (4, 4, 1, 899), (5, 1, 10, 299);

INSERT INTO kb_articles (title, body) VALUES
  ('Shipping times', 'Standard shipping takes 4 to 7 business days after production. You can follow your package with the tracking link in your shipping confirmation email.'),
  ('Refund policy', 'We refund an order if it arrives damaged or does not match the proof you approved. Refunds go back to the original payment method within 5 to 10 business days.'),
  ('Duplicate charges', 'A pending authorization can look like a second charge. It drops off your statement automatically within 3 business days if the order was only paid once.'),
  ('Changing your email or password', 'Open Account settings to update your email address. Use the reset link on the login page if you forgot your password.'),
  ('Choosing a sticker size and material', 'Die-cut vinyl stickers are waterproof and suit outdoor use. Holographic stickers are available in sizes from 1 to 4 inches.');

INSERT INTO campaigns (name, subject, status, sent_count, scheduled_for) VALUES
  ('Autumn restock', 'Your favorite stickers are back', 'sent', 1240, now() - interval '5 days'),
  ('Holographic launch', 'New: holographic finish', 'scheduled', 0, now() + interval '2 days'),
  ('Win-back', 'We saved your design', 'draft', 0, NULL);

INSERT INTO tickets (customer_id, subject, body, category, status) VALUES
  (2, 'Where is my package?', 'My order shipped 3 days ago but the tracking shows nothing new.', 'shipping', 'open');
