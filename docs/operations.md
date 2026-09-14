# Operação

## Subir o ambiente (qualquer modo)

```bash
az deployment group create \
  --resource-group <RESOURCE_GROUP> \
  --template-file infra/main.bicep \
  --parameters infra/parameters/economic.parameters.json \
  --parameters relayUser=$RELAY_USER relayPassword=$RELAY_PASSWORD \
               tlsCertPem="$(cat path/to/tls.crt)" tlsKeyPem="$(cat path/to/tls.key)"
```

Para o Modo B, trocar `economic.parameters.json` por
`dedicated.parameters.json` — nenhum outro arquivo muda.

## Derrubar o ambiente

```bash
az group delete --name <RESOURCE_GROUP> --yes
```

Como toda a infraestrutura é declarada em Bicep e os secrets nunca vivem
só no Azure (ficam documentados/versionados fora do Git conforme
[security.md](security.md)), o ambiente pode ser recriado do zero a
qualquer momento com o comando de "subir" acima — essa é a base do
modelo de infraestrutura efêmera pedido no plano.

## Descobrir o IP de saída atual (Modo A)

O endpoint de readiness (`/readyz`) não expõe o IP de saída diretamente.
Para diagnosticar qual IP a UOL (ou qualquer allowlist) veria, execute a
partir de dentro do Container App:

```bash
az containerapp exec --name <containerAppName> --resource-group <RESOURCE_GROUP> --command sh
# dentro do container (imagem distroless não tem shell -- ver nota abaixo)
```

**Nota**: a imagem final é `distroless` (sem shell), então `az containerapp
exec` não funciona diretamente nela. Formas práticas de checar o IP de
saída no Modo A:

1. Temporariamente rodar uma revisão/job auxiliar com uma imagem que tenha
   `curl` (ex.: `curlimages/curl`) na mesma VNet/subnet, e chamar
   `curl https://ifconfig.me`.
2. Ou, mais simples: como o Modo A já não garante IP limpo, tratar
   qualquer teste de IP como diagnóstico pontual, não como algo o relay
   precisa expor permanentemente — evita adicionar uma rota HTTP extra ao
   binário só para isso.

## Modo B: capturar o IP dedicado

Depois de um deploy com `enableDedicatedEgress: true`:

```bash
az deployment group show \
  --resource-group <RESOURCE_GROUP> \
  --name main \
  --query properties.outputs.dedicatedEgressIp.value -o tsv
```

Esse é o IP público estático do NAT Gateway (`infra/modules/nat-gateway.bicep`).
Ele **não muda** entre reinícios do Container App (é o mesmo recurso de IP
público, `Standard`/`Static`), mas muda se o ambiente inteiro for
destruído e recriado (`az group delete` + novo deploy) — nesse caso,
repetir este comando e atualizar qualquer allowlist externa que dependa
desse IP.

## Health checks e por que uma queda da UOL não derruba o relay

- `/healthz` (liveness, `src/relay/healthcheck.go`): sempre `200` enquanto
  o processo está de pé. É o único sinal usado para decidir reiniciar o
  container.
- `/readyz` (readiness): faz um dial TCP rápido para
  `SMTP_UPSTREAM_HOST:SMTP_UPSTREAM_PORT`. Se a UOL estiver indisponível,
  retorna `503` — isso tira o replica de rotação de tráfego, mas **nunca**
  é usado como gatilho de restart. Uma instabilidade temporária da UOL
  vira "não pronto", não "não vivo".

## Rotação de secrets

Trocar `RELAY_PASSWORD` (ex.: se a senha da conta UOL mudar) ou o
certificado TLS:

```bash
az containerapp secret set \
  --name <containerAppName> --resource-group <RESOURCE_GROUP> \
  --secrets relay-password=<nova-senha>
az containerapp revision restart --name <containerAppName> --resource-group <RESOURCE_GROUP>
```

Repetir o mesmo padrão para `tls-cert`/`tls-key`. Ver
[troubleshooting.md](troubleshooting.md) para o processo de renovação do
certificado, que ainda não está automatizado nesta versão.
