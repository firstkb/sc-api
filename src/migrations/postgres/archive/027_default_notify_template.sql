
INSERT INTO notification_template
(tenant_id, kind, key, locale, subject, html, text)
VALUES
(NULL, 'email', 'otp', 'default',
 'Our OTP Code',
 '<p>Hello!</p><p>Your OTP code: <b>{{ code }}</b></p><p>Code valid for {{ ttl }} minutes.</p>',
 'Hello!\nYour OTP code: {{ code }}\nCode valid for {{ ttl }} minutes.');

 INSERT INTO notification_template
(tenant_id, kind, key, locale, subject, html, text)
VALUES
(NULL, 'sms', 'otp', 'default',
 NULL,
 NULL,
 'Your code {{ code }}, valid for {{ ttl }} minutes.');