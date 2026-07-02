# T-Bank TLS CA Certificates

These public CA certificates are included in the backend image so the Go HTTP
client can verify T-Bank API certificates through the container trust store.

Official sources:

- Russian Trusted Root CA: https://gu-st.ru/content/Other/doc/russian_trusted_root_ca.cer
- Russian Trusted Sub CA: https://gu-st.ru/content/Other/doc/russian_trusted_sub_ca.cer

SHA-256 file checksums:

```text
936a43fea6e8e525bcc0f81acd9c3d21b4fc4b9b68acea7906d698005afc6504  russian_trusted_root_ca.crt
f0ae589f36774f29ef3648f7984b08d42fcce6f1ffeeb6236d773daeb2744ea6  russian_trusted_sub_ca.crt
```

To refresh them, download from the official sources above, verify the certificate
subjects and fingerprints with `openssl x509 -in <file> -noout -subject -issuer
-dates -fingerprint -sha256`, then update this checksum list.

The backend runtime base image already includes HARICA roots through Debian
`ca-certificates`. If a future base image no longer includes them, use the
official T-Bank HARICA bundle at https://developer.tbank.ru/harica.crt and keep
one certificate per `.crt` file so `update-ca-certificates` can rehash them
cleanly.
