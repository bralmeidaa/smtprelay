# Segurança

## Garantias contra open relay

1. **Autenticação obrigatória**: `MAIL FROM` sem `AUTH` bem-sucedido antes
   é rejeitado com `530` ([`src/relay/server.go`](../src/relay/server.go),
   `Session.Mail`). Provado por
   [`tests/no_auth_rejected_test.go`](../tests/no_auth_rejected_test.go).
2. **Uma única credencial válida**: usuário e senha são fixos
   (`RELAY_USER`/`RELAY_PASSWORD`), comparados em tempo constante
   (`crypto/subtle`). Qualquer outra credencial é rejeitada — não existe
   caminho de configuração que aceite múltiplos usuários ou credenciais
   parciais. Provado por
   [`tests/auth_invalid_test.go`](../tests/auth_invalid_test.go).
3. **`MAIL FROM` deve bater com a conta autenticada**: mesmo autenticado,
   o cliente não pode informar um remetente diferente da conta com que se
   autenticou (`553`). Isso fecha a lacuna entre "não é open relay
   clássico" e "não vira vetor de spoofing por um dos 4 usuários". Provado
   por
   [`tests/open_relay_blocked_test.go`](../tests/open_relay_blocked_test.go).
4. **TLS obrigatório nas duas pontas**: `AllowInsecureAuth: false` — o
   relay nunca anuncia nem aceita `AUTH` antes do `STARTTLS`/TLS implícito.
   A conexão de saída para a UOL sempre valida o certificado do servidor
   (`crypto/tls`, sem `InsecureSkipVerify`). Provado por
   [`tests/tls_test.go`](../tests/tls_test.go).

## Portas e TLS do lado do cliente

O relay aceita duas formas de TLS, pela mesma decisão tomada antes da
implementação (nem todo cliente de e-mail suporta STARTTLS):

- **587 — STARTTLS** (`RFC 3207`): conexão em texto claro que faz upgrade
  para TLS antes de qualquer comando sensível.
- **465 — TLS implícito** (`RFC 8314`, "SMTPS"): TLS desde o primeiro
  byte.

Em ambas, `MinVersion: TLS 1.2` e nenhum cipher/verificação é relaxado.

### Certificado do relay

O binário Go carrega o certificado/chave a partir de arquivos
(`TLS_CERT_FILE`/`TLS_KEY_FILE`) — no Container Apps, montados via um
volume de secret (ver `infra/modules/containerapp.bicep`). O Container
Apps não termina TLS para ingress TCP puro (isso só existe para ingress
HTTP), então **o TLS de `smtps.bratech.me` é responsabilidade do próprio
binário**, não da plataforma.

### Emissão e renovação (Let's Encrypt via ACME)

Como o relay só expõe TCP puro (587/465, sem HTTP ingress), os desafios
ACME `HTTP-01` e `TLS-ALPN-01` não se aplicam — só `DNS-01` funciona, e é
justamente o que não depende de nenhuma porta do relay estar aberta.

[`.github/workflows/renew-cert.yml`](../.github/workflows/renew-cert.yml)
automatiza isso, rodando duas vezes por mês (bem dentro da validade de 90
dias de um certificado Let's Encrypt) **e também logo depois de todo
deploy bem-sucedido** (`workflow_run` disparado pelo `ci-cd.yml`) — assim
o relay nunca fica rodando com o certificado autoassinado de bootstrap por
mais do que o tempo de um deploy:

1. `lego --dns namedotcom` cria um registro `TXT` temporário em
   `_acme-challenge.smtps.bratech.me` via API do Name.com para provar
   posse do domínio, e emite um certificado novo.
2. `az containerapp secret set` atualiza os secrets `tls-cert`/`tls-key`
   do Container App com o novo par.
3. A revisão ativa é reiniciada para carregar o certificado novo (se não
   houver réplica rodando — `minReplicas: 0` — o próximo cold start já
   nasce com o certificado atualizado, sem necessidade de restart).

Como o DNS-01 não depende de estado anterior, o workflow simplesmente
emite um certificado **novo** a cada execução, em vez de tentar rastrear
"renovar só se necessário" entre execuções efêmeras do GitHub Actions —
bem abaixo do limite de rate limit da Let's Encrypt nessa frequência.

Requer os secrets `NAMECOM_USERNAME`, `NAMECOM_API_TOKEN` (gerados em
Name.com → Account Settings → API) e `LETSENCRYPT_EMAIL`, além das
variables `AZURE_CLIENT_ID`/`AZURE_TENANT_ID`/`AZURE_SUBSCRIPTION_ID`
(OIDC) já usadas no deploy.

**Multi-domínio**: o workflow roda como uma matrix — cada domínio do
Name.com que precisar de certificado é uma entrada em `matrix.site`
(`domain`, `containerApp`, `resourceGroup`), não um workflow separado. O
nome do resource group e do Container App não são secret (só nomes), por
isso ficam direto na matrix. Isso só serve enquanto todos os domínios
estiverem na mesma conta Name.com e na mesma assinatura/identidade Azure
que os secrets/variables acima autorizam — um domínio que precise de outra
conta Name.com ou outra assinatura Azure não cabe nesse modelo de
credenciais compartilhadas e exigiria secrets próprios.

## Credenciais e secrets

- `RELAY_USER`/`RELAY_PASSWORD` (== e-mail e senha reais da UOL) e o
  certificado/chave TLS existem **apenas** como Container Apps secrets
  (`infra/modules/containerapp.bicep` → `properties.configuration.secrets`),
  nunca em texto puro no Bicep, nos parâmetros JSON, no Dockerfile ou no
  código.
- `.env.example` só tem placeholders — `.env` real está no `.gitignore`.
- A senha da UOL trafega através do relay (cliente → relay → UOL). Por
  isso o TLS obrigatório nas duas pontas não é opcional nem
  configurável para menos.

## O que nunca é logado

`src/relay/logger.go` centraliza os logs estruturados (`log/slog`, JSON).
Regra: **nenhuma senha, em nenhum nível**, nem o corpo/conteúdo integral
das mensagens. O e-mail do usuário é mascarado antes de logar
(`maskEmail`: `cliente@uol.com.br` → `c****e@uol.com.br`) — suficiente
para troubleshooting sem expor o endereço completo em texto claro nos
logs.

## Container

- Imagem final `gcr.io/distroless/static-debian12:nonroot` — sem shell,
  sem gerenciador de pacotes, sem nada além do binário estático.
- Processo roda como usuário `nonroot` (`USER nonroot:nonroot` no
  `Dockerfile`), nunca como root.
- Build multi-stage: a etapa de compilação (`golang:1.23-alpine`) não vai
  para a imagem final.

## Rate limiting e limites

Implementado em [`src/relay/ratelimit.go`](../src/relay/ratelimit.go),
por IP de origem:

| Limite | Padrão | Variável |
|---|---|---|
| Conexões/minuto | 30 | `RATE_LIMIT_CONN_PER_MINUTE` |
| Mensagens/hora | 200 | `RATE_LIMIT_MSGS_PER_HOUR` |
| Tamanho máx. de mensagem | 15 MB | `MAX_MESSAGE_SIZE_BYTES` |
| Destinatários por mensagem | 20 | `MAX_RECIPIENTS` |

Esses defaults são um ponto de partida proporcional a ~4 usuários de baixo
volume, não um número validado com o cliente — ajustar em
`infra/modules/containerapp.bicep` (ou via env var local) se a UOL impuser
limites mais apertados ou se o uso real mostrar que estão errados.

Nota técnica importante sobre o limite de conexões: o `go-smtp` só invoca
a checagem de rate limit no primeiro `EHLO`/`HELO` de uma conexão, e o
protocolo exige um **novo** `EHLO` depois do `STARTTLS` (o estado anterior
é descartado, por segurança — RFC 3207). Isso significa que uma sessão
completa via STARTTLS consome **dois** "créditos" de conexão, não um. Os
valores padrão já têm folga para isso; é só ter em mente ao ajustar
`RATE_LIMIT_CONN_PER_MINUTE` para baixo.

## Timeouts e graceful shutdown

`READ_TIMEOUT`/`WRITE_TIMEOUT` derrubam conexões ociosas ou travadas.
`UPSTREAM_TIMEOUT` limita quanto tempo o relay espera pela UOL antes de
responder `451` (falha temporária) ao cliente. No `SIGTERM`/`SIGINT`
(`src/relay/run.go`), o relay para de aceitar novas conexões e dá até 20s
para as conexões em andamento terminarem antes de encerrar.
