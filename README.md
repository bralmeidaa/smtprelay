# smtprelay

Relay SMTP simples, seguro e de baixo custo na Azure, para tirar o IP
residencial/compartilhado do caminho de conexão final com a UOL — sem
recriar o mesmo problema com um pool de egress igualmente "sujo".

O cliente conecta o app de e-mail em `smtps.bratech.me` (porta 587
STARTTLS ou 465 TLS implícito) usando as **mesmas credenciais** da conta
`@uol.com.br`. O relay autentica a sessão e repassa a mensagem, autenticado
e via STARTTLS, para `smtps.uol.com.br:587`.

Ver [docs/architecture.md](docs/architecture.md) para o desenho completo
(incluindo por que a VNet é obrigatória nos dois modos) e
[docs/cost.md](docs/cost.md) para os números reais de custo por modo.

## Escopo

**Não implementa** servidor de e-mail completo, IMAP/POP3, webmail,
mailbox, antispam/antivírus completo, AKS/Kubernetes, ou alta
disponibilidade complexa. O Static Web App do mesmo cliente é um
repositório separado
([bralmeidaa/staticwebapp-est](https://github.com/bralmeidaa/staticwebapp-est))
e está fora de escopo aqui.

## Estrutura

```
smtprelay/
├── src/
│   ├── main.go              # entrypoint (package main); só chama relay.Run()
│   └── relay/                # toda a lógica -- package separado porque Go
│       ├── config.go         # não permite importar "package main" em testes
│       ├── auth.go           # externos (ver tests/), então a lógica mora
│       ├── ratelimit.go      # aqui e main.go vira um wrapper fino.
│       ├── upstream.go
│       ├── healthcheck.go
│       ├── server.go
│       ├── logger.go
│       └── run.go
├── tests/                    # testes de integração ponta a ponta (SMTP real
│                              # sobre TCP local), não testes unitários isolados
├── infra/                    # Bicep -- ver docs/architecture.md
├── docs/
└── .github/workflows/ci-cd.yml
```

**Desvio deliberado da estrutura originalmente planejada**: o plano listava
os arquivos `.go` direto em `src/`, sem `main.go` nem subpasta `relay/`.
Isso não é possível em Go sem quebrar os testes: `package main` não pode
ser importado por nenhum outro pacote (nem por testes em `tests/`), então
a lógica precisou ir para um pacote `relay` importável, com `main.go`
como um wrapper fino em `package main`. Mesmos arquivos, mesma
responsabilidade de cada um — só um nível de diretório a mais.

## Rodando localmente

```bash
cp .env.example .env
# editar .env com credenciais reais e um certificado TLS de teste
go run ./src
```

## Testes

```bash
go test ./... -v
```

Cobrem: autenticação válida/inválida, rejeição sem autenticação, prova
explícita de que não é open relay (remetente forjado é bloqueado mesmo
autenticado), TLS (STARTTLS + implícito, e rejeição de AUTH antes do TLS),
upstream indisponível, timeout de conexão ociosa, rate limit (conexão e
mensagem), tamanho máximo de mensagem, graceful shutdown, reconexão.

## Build da imagem

```bash
docker build -t smtprelay:local .
```

Multi-stage: compila em `golang:1.23-alpine`, roda em
`distroless/static-debian12:nonroot` (sem shell, usuário não-root).

## Deploy

Ver [docs/operations.md](docs/operations.md). Resumo:

```bash
az deployment group create \
  --resource-group <RESOURCE_GROUP> \
  --template-file infra/main.bicep \
  --parameters infra/parameters/economic.parameters.json \
  --parameters relayUser=$RELAY_USER relayPassword=$RELAY_PASSWORD \
               tlsCertPem="$(cat tls.crt)" tlsKeyPem="$(cat tls.key)"
```

Troque `economic.parameters.json` por `dedicated.parameters.json` para o
Modo B (IP dedicado, custo real — ver [docs/cost.md](docs/cost.md)).

## Documentação

- [architecture.md](docs/architecture.md) — os dois modos, diagramas, por
  que a VNet é obrigatória.
- [security.md](docs/security.md) — garantias contra open relay, TLS,
  secrets, o que nunca é logado.
- [cost.md](docs/cost.md) — números reais por modo, comparação com VM.
- [dns.md](docs/dns.md) — hostname do relay e da UOL, validado por teste
  de conexão real.
- [operations.md](docs/operations.md) — subir/derrubar ambiente, rotação
  de secrets, health checks.
- [troubleshooting.md](docs/troubleshooting.md)
- [rollback.md](docs/rollback.md)

## Status / pendências conhecidas

**Deployado e rodando de verdade** (2026-09-15, Brazil South, Modo A) —
não é mais só teoria/lint local. Nessa sessão:

- Pipeline completo verde ponta a ponta: build, testes, `govulncheck`,
  build/push da imagem pro Docker Hub, deploy do Bicep, renovação do
  certificado real via Let's Encrypt e criação automática do DNS.
- Três problemas reais só apareceram no deploy de verdade (não tinha como
  prever por lint/documentação sozinha) e foram corrigidos:
  - Subnet da infra precisa de delegação explícita pro serviço
    `Microsoft.App/environments` — ver
    [docs/troubleshooting.md](docs/troubleshooting.md#o-deploy-do-bicep-falha-com-erro-de-ingressvnet).
  - East US estava sem capacidade de AKS pra novos ambientes
    Container Apps workload-profiles (`AKSCapacityHeavyUsage`) — trocado
    o default pra Brazil South, com preços revalidados em
    [docs/cost.md](docs/cost.md).
  - `govulncheck` exige Go 1.26+ pra instalar, e o workflow tinha
    `go-version: '1.23'` fixo — trocado pra `'stable'`.
- Renovação do certificado TLS
  ([`.github/workflows/renew-cert.yml`](.github/workflows/renew-cert.yml),
  Let's Encrypt via DNS-01 no Name.com, suporta múltiplos
  domínios/Container Apps na mesma matrix) já rodou com sucesso e
  substituiu o certificado autoassinado de bootstrap pelo real. Também
  dispara automaticamente logo depois de todo deploy bem-sucedido
  (`workflow_run`), além do agendamento quinzenal — ver
  [docs/security.md](docs/security.md#emissão-e-renovação-lets-encrypt-via-acme).
- DNS (`smtps.bratech.me` → FQDN do Container App) também é automático,
  criado/atualizado a cada deploy via API do Name.com — ver
  [docs/dns.md](docs/dns.md). Confirmado que "custom domains" do
  Container Apps não se aplica a ingress TCP (é feature só de HTTP
  ingress), então não há nada a configurar nesse sentido.

Ainda pendente:

- Teste funcional ponta a ponta com um cliente de e-mail real (Outlook/
  Thunderbird/app do celular) apontando pra `smtps.bratech.me` — o que foi
  validado até aqui é a infraestrutura subindo e o certificado sendo
  aplicado, não uma sessão SMTP real de um cliente de e-mail de verdade.
- Modo B (NAT Gateway) nunca foi deployado, só o Modo A.
