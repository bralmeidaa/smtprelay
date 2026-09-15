# DNS

## Hostname do relay (lado cliente)

`smtps.bratech.me` — domínio hospedado no Name.com.

Automatizado: o step "Point DNS at the deployed Container App" em
[`ci-cd.yml`](../.github/workflows/ci-cd.yml) roda depois de cada deploy
bem-sucedido, pega o FQDN público do Container App (saída
`containerAppFqdn` do `infra/main.bicep`) e cria/atualiza um registro
`CNAME` de `smtps` → esse FQDN via API do Name.com (v4, mesmas
credenciais `NAMECOM_USERNAME`/`NAMECOM_API_TOKEN` usadas pro
`renew-cert.yml`). Não precisa mexer manualmente no DNS depois de um
deploy.

**Confirmado**: `az containerapp hostname bind`/"custom domains" é uma
feature de ingress **HTTP** (SNI + certificado gerenciado pela Azure) —
não se aplica a ingress TCP puro como o nosso. Pra TCP, o CNAME sozinho
já é suficiente; o TLS de `smtps.bratech.me` continua sendo resolvido
inteiramente pelo certificado carregado dentro do próprio binário (ver
[security.md](security.md)), nunca pela Azure.

Por que CNAME e não `A` com IP fixo: o FQDN do Container App não muda
mesmo que a Azure troque o IP por baixo dos panos — um registro `A`
hardcoded quebraria nesse cenário.

## Hostname upstream (UOL)

Confirmado via teste de conexão real em 2026-09-14:

```
openssl s_client -starttls smtp -connect smtps.uol.com.br:587
```

Resultado: handshake TLS bem-sucedido, certificado válido (`CN=smtps.uol.com.br`,
emitido pela DigiCert, válido até 2027-02-04), respostas SMTP normais
(`250 CHUNKING`, `221 2.0.0 Bye`).

**Conclusão**: o valor padrão `smtpUpstreamHost=smtps.uol.com.br`,
`smtpUpstreamPort=587` (STARTTLS) está correto. Não é necessário trocar
para `smtp.uol.com.br`.

### Processo de troca (se a UOL mudar algo no futuro)

`smtpUpstreamHost`/`smtpUpstreamPort` são variáveis de configuração
(`infra/modules/containerapp.bicep` → env vars `SMTP_UPSTREAM_HOST`/
`SMTP_UPSTREAM_PORT`), nunca hardcoded no código Go. Trocar é só:

1. Repetir o teste `openssl s_client -starttls smtp -connect <novo-host>:<porta>`
   para confirmar STARTTLS.
2. Atualizar o parâmetro em `infra/parameters/*.parameters.json`.
3. Rodar o pipeline/`az deployment group create` de novo — sem tocar no
   código.
