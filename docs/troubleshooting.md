# Troubleshooting

## Cliente não consegue autenticar

- Confirmar que o app de e-mail está usando **exatamente** o e-mail e a
  senha reais da conta `@uol.com.br` (`RELAY_USER`/`RELAY_PASSWORD` no
  relay devem bater com o que o cliente digita).
- Confirmar que o cliente está usando STARTTLS na 587 ou TLS
  implícito/SSL na 465 — o relay rejeita `AUTH` em texto claro
  (`AllowInsecureAuth: false`), então um cliente configurado sem TLS trava
  silenciosamente ou recebe erro genérico de autenticação.
- Ver logs (`az containerapp logs show`) por `"authentication failed"` —
  o campo `user` vem mascarado (`c****e@uol.com.br`), mas confirma que a
  tentativa chegou no relay.

## Mensagens não chegam ao destinatário

1. Checar `/readyz` — se estiver `503`, a UOL está inacessível
   (rede/upstream fora do ar), não é bug do relay.
2. Ver logs por `"upstream relay failed"` — a mensagem de erro
   (`upstream dial`, `upstream auth`, `upstream starttls`, etc.) indica em
   qual etapa da conexão com a UOL falhou.
3. Se for `upstream auth`: a senha configurada no relay pode estar
   desatualizada em relação à senha real da conta UOL — ver
   [operations.md](operations.md#rotação-de-secrets).

## Erro "553 sender address does not match authenticated account"

Comportamento esperado (ver [security.md](security.md)) — o cliente está
tentando enviar com um remetente diferente do e-mail com que se
autenticou. Corrigir o campo "De" no app de e-mail para bater com a conta
configurada.

## Erro "452 message rate limit exceeded" / "421 too many connections"

Os limites padrão (`RATE_LIMIT_CONN_PER_MINUTE=30`,
`RATE_LIMIT_MSGS_PER_HOUR=200`) são proporcionais a ~4 usuários de baixo
volume — se isso disparar em uso normal, os valores provavelmente estão
baixos demais para o padrão real de uso; ajustar em
`infra/modules/containerapp.bicep` (ou via env var) e reimplantar.

## Certificado TLS expirado

A renovação é automática via
[`.github/workflows/renew-cert.yml`](../.github/workflows/renew-cert.yml)
(ver [security.md](security.md#emissão-e-renovação-lets-encrypt-via-acme)),
rodando duas vezes por mês. Se mesmo assim um cliente rejeitar a conexão
TLS por certificado expirado:

1. Checar as execuções do workflow em Actions — falha mais provável é
   `NAMECOM_API_TOKEN` expirado/revogado ou o registro `TXT` do desafio
   não propagando a tempo (`NAMECOM_PROPAGATION_TIMEOUT`).
2. Rodar o workflow manualmente (`workflow_dispatch`) para forçar uma
   renovação imediata.
3. Se precisar de um certificado emergencial sem esperar o workflow:
   gerar manualmente (`lego` local ou qualquer cliente ACME com DNS-01) e
   atualizar os secrets `tls-cert`/`tls-key` na mão (ver
   [operations.md](operations.md#rotação-de-secrets)).

## O deploy do Bicep falha com erro de ingress/VNet

A arquitetura depende de um ambiente Container Apps "workload profiles"
com VNet integrada (ver [architecture.md](architecture.md)). Um erro já
apareceu e foi corrigido no primeiro deploy real:

- **"The subnet of the environment must be delegated to the service
  'Microsoft.App/environments'"** — a subnet de infraestrutura precisa de
  delegação explícita pro serviço `Microsoft.App/environments`, mesmo num
  ambiente workload-profiles/VNet (não estava claro na documentação usada
  para desenhar a arquitetura). Corrigido em
  `infra/modules/networking.bicep` (e replicado em
  `infra/modules/nat-gateway.bicep`, que redeclara a mesma subnet ao
  anexar o NAT Gateway — sem repetir a delegação lá, o Modo B apagaria
  ela).

Se aparecer outro erro de ingress/VNet:

1. Conferir a versão da API (`Microsoft.App/managedEnvironments@...`,
   `Microsoft.App/containerApps@...`) contra a mais recente disponível na
   região.
2. Conferir o tamanho mínimo exigido para a subnet de infraestrutura
   (`infraSubnetAddressPrefix` em `infra/modules/networking.bicep`) — a
   documentação da Azure já mudou esse requisito mais de uma vez.
3. Reportar o erro exato — provavelmente é mais um ajuste pontual no
   Bicep, não um problema de arquitetura.

## Container reinicia em loop

Não deve acontecer por causa da UOL estar fora do ar (ver
[operations.md](operations.md#health-checks-e-por-que-uma-queda-da-uol-não-derruba-o-relay)).
Se acontecer mesmo assim, checar `/healthz` diretamente e os logs de
inicialização — a causa mais provável é `RELAY_USER`/`RELAY_PASSWORD` ou
`TLS_CERT_FILE`/`TLS_KEY_FILE` ausentes ou inválidos, que fazem o processo
sair com código 1 logo no start (`src/relay/run.go`).
