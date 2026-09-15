// SMTP relay infrastructure -- two modes, one switch.
//
// Modo A (default, enableDedicatedEgress = false): VNet-integrated
// Container Apps environment (mandatory for raw TCP ingress -- see
// docs/architecture.md), Consumption workload profile, scale-to-zero. No
// NAT Gateway: egress uses the platform's shared SNAT pool, so the outbound
// IP is NOT guaranteed clean. Cost stays near-zero.
//
// Modo B (enableDedicatedEgress = true): adds a NAT Gateway on the same
// VNet for a single, stable, dedicated egress IP. Real ongoing cost -- see
// docs/cost.md. Only turn this on once the client has accepted that cost.
//
// Flipping enableDedicatedEgress is the only thing that should ever be
// needed to switch modes; nothing else in this template should require
// editing to do it.

targetScope = 'resourceGroup'

@description('Azure region for every resource.')
param location string = 'brazilsouth'

@description('Name of the Container Apps environment.')
param environmentName string

@description('Name of the container app running the relay.')
param containerAppName string

@description('Docker Hub image reference, e.g. docker.io/bralmeida/smtprelay:latest.')
param containerImage string

param cpu string = '0.25'
param memory string = '0.5Gi'

@minValue(0)
param minReplicas int = 0

@minValue(1)
param maxReplicas int = 1

@description('UOL upstream SMTP host. Confirmed via real connection test -- see docs/dns.md.')
param smtpUpstreamHost string = 'smtps.uol.com.br'

param smtpUpstreamPort string = '587'

@description('Public hostname clients point their mail app at.')
param relayDomain string = 'smtps.bratech.me'

param maxMessageSizeBytes string = '15728640'
param maxRecipients string = '20'
param readTimeout string = '30s'
param writeTimeout string = '60s'
param upstreamTimeout string = '30s'
param rateLimitConnPerMinute string = '30'
param rateLimitMsgsPerHour string = '200'

@description('Turns on Modo B: NAT Gateway for a dedicated, stable egress IP. false = Modo A (default).')
param enableDedicatedEgress bool = false

param vnetName string = '${environmentName}-vnet'
param vnetAddressPrefix string = '10.20.0.0/23'
param infraSubnetAddressPrefix string = '10.20.0.0/24'

@secure()
@description('The single fixed relay account -- also the real UOL mailbox credentials. Pass via a secret parameter file or CI secret, never committed.')
param relayUser string

@secure()
param relayPassword string

@secure()
@description('PEM-encoded TLS certificate for relayDomain (client-facing STARTTLS/implicit TLS).')
param tlsCertPem string

@secure()
param tlsKeyPem string

module networking 'modules/networking.bicep' = {
  name: 'networking'
  params: {
    location: location
    vnetName: vnetName
    vnetAddressPrefix: vnetAddressPrefix
    infraSubnetAddressPrefix: infraSubnetAddressPrefix
  }
}

module natGateway 'modules/nat-gateway.bicep' = if (enableDedicatedEgress) {
  name: 'nat-gateway'
  params: {
    location: location
    natGatewayName: '${environmentName}-natgw'
    publicIpName: '${environmentName}-egress-ip'
    vnetName: vnetName
    infraSubnetName: networking.outputs.infraSubnetName
  }
}

module containerApp 'modules/containerapp.bicep' = {
  name: 'container-app'
  params: {
    location: location
    environmentName: environmentName
    containerAppName: containerAppName
    infraSubnetId: networking.outputs.infraSubnetId
    containerImage: containerImage
    cpu: cpu
    memory: memory
    minReplicas: minReplicas
    maxReplicas: maxReplicas
    smtpUpstreamHost: smtpUpstreamHost
    smtpUpstreamPort: smtpUpstreamPort
    relayDomain: relayDomain
    maxMessageSizeBytes: maxMessageSizeBytes
    maxRecipients: maxRecipients
    readTimeout: readTimeout
    writeTimeout: writeTimeout
    upstreamTimeout: upstreamTimeout
    rateLimitConnPerMinute: rateLimitConnPerMinute
    rateLimitMsgsPerHour: rateLimitMsgsPerHour
    relayUser: relayUser
    relayPassword: relayPassword
    tlsCertPem: tlsCertPem
    tlsKeyPem: tlsKeyPem
  }
  dependsOn: enableDedicatedEgress ? [natGateway] : []
}

output containerAppFqdn string = containerApp.outputs.containerAppFqdn
// natGateway only exists when enableDedicatedEgress is true, which is
// exactly the condition guarding this access -- safe despite the BCP318
// warning Bicep's static analysis can't resolve for conditional modules.
#disable-next-line BCP318
output dedicatedEgressIp string = enableDedicatedEgress ? natGateway.outputs.dedicatedEgressIp : ''
output mode string = enableDedicatedEgress ? 'B (dedicated egress)' : 'A (economic)'
