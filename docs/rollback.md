# Rollback

## Nível 1 — Revisão do Container App

Container Apps mantém revisões anteriores por padrão. Se um deploy novo
quebrar algo:

```bash
az containerapp revision list --name <containerAppName> --resource-group <RESOURCE_GROUP> -o table
az containerapp ingress traffic set \
  --name <containerAppName> --resource-group <RESOURCE_GROUP> \
  --revision-weight <revisao-anterior>=100
```

Mais rápido que refazer o deploy — volta o tráfego para a última revisão
saudável em segundos.

## Nível 2 — Imagem anterior no Docker Hub

Toda imagem publicada em `bralmeida/smtprelay` é taggeada com o SHA do
commit (ver `.github/workflows/ci-cd.yml`), além de `latest`. Para reverter
o código para uma versão específica:

```bash
az deployment group create \
  --resource-group <RESOURCE_GROUP> \
  --template-file infra/main.bicep \
  --parameters infra/parameters/economic.parameters.json \
  --parameters containerImage=docker.io/bralmeida/smtprelay:<sha-anterior> \
               relayUser=$RELAY_USER relayPassword=$RELAY_PASSWORD \
               tlsCertPem="$(cat tls.crt)" tlsKeyPem="$(cat tls.key)"
```

## Nível 3 — Infraestrutura inteira

Como tudo é declarado em `infra/main.bicep` (infraestrutura efêmera, ver
[operations.md](operations.md)), o rollback mais bruto é sempre disponível:
apagar o resource group e reimplantar do zero a partir do último commit
bom em `main`.

```bash
az group delete --name <RESOURCE_GROUP> --yes
# depois, "Subir o ambiente" em operations.md
```

## O que rollback NÃO cobre

- **Secrets**: revisões antigas apontam para os mesmos secrets atuais do
  Container App (`relay-user`, `relay-password`, `tls-cert`, `tls-key`) —
  reverter a revisão/imagem não desfaz uma rotação de secret. Se o
  problema foi uma senha ou certificado trocado incorretamente, reverter o
  secret também (ver
  [operations.md](operations.md#rotação-de-secrets)), não só a imagem.
- **DNS**: o registro apontando `smtps.bratech.me` para o FQDN do
  Container App não muda com rollback de revisão/imagem — só muda se o
  FQDN do ambiente mudar (ex.: recriação completa do ambiente no Nível 3).
