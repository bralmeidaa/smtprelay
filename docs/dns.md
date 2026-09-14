# DNS

## Hostname do relay (lado cliente)

`smtps.bratech.me` — domínio hospedado no Name.com.

Depois do primeiro deploy, criar um registro `CNAME` (ou `A`, se preferir
um IP fixo) apontando `smtps.bratech.me` para o FQDN público que o
Container App expõe (saída `containerAppFqdn` do `infra/main.bicep`).

**Pendente de validação real**: como o Container Apps não termina TLS para
ingress TCP puro, o binding de domínio customizado usado normalmente para
apps HTTP (`az containerapp hostname add` + certificado gerenciado) não se
aplica da mesma forma aqui — o registro DNS só precisa apontar para o
FQDN/IP do Container App; o TLS de `smtps.bratech.me` é resolvido pelo
certificado carregado dentro do próprio binário (ver
[security.md](security.md)). Confirmar o FQDN/IP exato exposto pelo modo
TCP ingress no primeiro deploy real e ajustar este documento.

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
