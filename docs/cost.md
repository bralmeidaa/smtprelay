# Custo

Todos os valores abaixo vêm da
[Azure Retail Prices API](https://prices.azure.com/api/retail/prices),
consultados em 2026-09-14/15 para a região **`brazilsouth`** (trocada de
East US depois de um deploy real esbarrar em falta de capacidade de AKS
lá — ver [architecture.md](architecture.md)) — não são estimativas
inventadas. Ainda assim, **confirmar na Calculadora de Preços da Azure**
antes de decidir com o cliente, já que preços mudam.

## Modo A — Econômico

### Compute (Container Apps, Consumption workload profile)

| Item | Preço |
|---|---|
| vCPU ativo | $0.000024 / vCPU-segundo |
| Memória ativa | $0.000003 / GiB-segundo |
| Grátis por assinatura/mês | 180.000 vCPU-segundos + 360.000 GiB-segundos + 2M requisições HTTP |

Com `minReplicas: 0` e ~4 usuários de volume muito baixo, é bem provável
que o uso mensal fique **dentro da faixa gratuita** — ou seja, Modo A pode
custar efetivamente **$0/mês** em compute na prática.

### A VNet não adiciona custo de gerenciamento

Ponto que verificamos explicitamente, porque a arquitetura corrigida
exige VNet em ambos os modos (ver [architecture.md](architecture.md)): a
documentação oficial de billing é explícita —

> "You aren't billed any plan management charges unless you use a
> Dedicated workload profile in your environment."
> — [Billing in Azure Container Apps](https://learn.microsoft.com/en-us/azure/container-apps/billing)

Como usamos só o workload profile `Consumption` (nunca um profile
`Dedicated`), a taxa de gerenciamento de ~$0.10/hora (~$73/mês) que existe
para profiles dedicados **não se aplica**. A VNet em si (sem NAT Gateway)
não tem custo de recurso próprio — só os componentes de Modo B (abaixo)
custam.

**Total Modo A estimado: ~$0/mês**, sujeito a validação com uso real.

## Modo B — IP dedicado

Componentes adicionais sobre o Modo A:

| Item | Preço | ~Mensal (730h) |
|---|---|---|
| NAT Gateway (Standard) — gateway hours | $0.045/hora | ~$32.85 |
| NAT Gateway (Standard) — dados processados | $0.045/GB | ~$0.20–1 (volume muito baixo) |
| IP público estático (Standard) | $0.005/hora | ~$3.65 |
| **Total Modo B** | | **~$36–38/mês** |

O preço do NAT Gateway retornado pela API não é específico de região
(meter global, mesmo valor em `brazilsouth` e `eastus`) — confirmar na
calculadora ao ativar o Modo B, mas a ordem de grandeza (~$0.045/h +
$0.045/GB) é a mesma usada publicamente pela Azure para essa SKU há anos.
A IP pública estática também não muda de preço entre as duas regiões
($0.005/hora).

## Comparação com alternativa: VM Linux pequena

O plano original pede essa comparação explicitamente (Fase 8). Resultado,
usando uma `Standard_B1s` (1 vCPU, 1 GiB) como proxy de "menor VM viável":

| Item | Preço | ~Mensal (730h) |
|---|---|---|
| VM `Standard_B1s` Linux (Brazil South) | $0.0168/hora | ~$12.26 |
| IP público estático (Standard) | $0.005/hora | ~$3.65 |
| Disco gerenciado (estimado, não medido) | — | ~$1–2 |
| **Total VM** | | **~$17–18/mês** |

Nota: o B1s em Brazil South custa ~62% mais que em East US ($0.0168/h vs.
$0.0104/h) — compute em geral é mais caro nessa região. O compute do
Container Apps, porém, **não varia** entre as duas regiões
($0.000024/vCPU-s e $0.000003/GiB-s iguais em ambas).

### Achado que vale destacar

**O Modo B (NAT Gateway) sai mais caro que rodar em uma VM pequena**, mesmo em Brazil South —
~$36–38/mês contra ~$17–18/mês. A VM, porém, roda o tempo todo (sem
`minReplicas: 0`), exige patch de SO e perde a vantagem de infraestrutura
efêmera (subir/derrubar sob demanda) que motivou a escolha por Container
Apps desde o início.

Isso não muda a recomendação de manter Container Apps como base (a
arquitetura em dois modos, o scale-to-zero e a natureza efêmera continuam
valendo a pena para Modo A), mas é um dado real para a conversa com o
cliente **antes** de ativar o Modo B: se o objetivo é só um IP de saída
dedicado e não há necessidade de escalar a zero, uma VM pequena com IP
estático é, hoje, a opção mais barata para resolver especificamente o
problema do IP sujo — a Container Apps + NAT Gateway.

## Resumo para decisão

| | Modo A | Modo B (NAT Gateway) | VM pequena |
|---|---|---|---|
| Custo mensal estimado | ~$0 | ~$36–38 | ~$17–18 |
| IP de saída garantido limpo | Não | Sim | Sim |
| Escala a zero | Sim | Sim | Não |
| Infra efêmera (subir/derrubar) | Sim | Sim | Parcial (recria VM) |
| Manutenção de SO | Não (distroless) | Não (distroless) | Sim |

**Recomendação**: validar tudo em Modo A primeiro (custo ~$0). Só decidir
entre Modo B e a VM pequena depois que o cliente confirmar que precisa de
IP dedicado — e, nesse momento, mostrar as duas opções, não só o Modo B.
