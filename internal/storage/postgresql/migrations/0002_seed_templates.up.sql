INSERT INTO templates (slug, name, device_type, window_width, window_height, user_agent, country_code, custom_headers) VALUES
  ('desktop-chrome-us', 'Desktop Chrome (US)', 'desktop', 1920, 1080,
   'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36',
   'US', '[{"key":"Accept-Language","value":"en-US,en;q=0.9"}]'::jsonb),
  ('desktop-firefox-de', 'Desktop Firefox (DE)', 'desktop', 1920, 1080,
   'Mozilla/5.0 (X11; Linux x86_64; rv:124.0) Gecko/20100101 Firefox/124.0',
   'DE', '[{"key":"Accept-Language","value":"de-DE,de;q=0.9"}]'::jsonb),
  ('mobile-iphone-us', 'Mobile iPhone (US)', 'mobile', 390, 844,
   'Mozilla/5.0 (iPhone; CPU iPhone OS 17_4 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.4 Mobile/15E148 Safari/604.1',
   'US', '[{"key":"Accept-Language","value":"en-US,en;q=0.9"}]'::jsonb),
  ('mobile-android-br', 'Mobile Android (BR)', 'mobile', 412, 915,
   'Mozilla/5.0 (Linux; Android 14; Pixel 8) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Mobile Safari/537.36',
   'BR', '[{"key":"Accept-Language","value":"pt-BR,pt;q=0.9"}]'::jsonb)
ON CONFLICT (slug) DO NOTHING;
