// Container Apps Environment (workload-profiles, VNet-integrated -- see
// ../../docs/architecture.md for why this is mandatory even in Modo A) and
// the relay Container App itself: dual raw-TCP ingress (587 STARTTLS + 465
// implicit TLS), scale-to-zero Consumption profile, secrets for
// credentials, and liveness/readiness probes that never treat a temporary
// UOL outage as a reason to restart.

@description('Region for the environment and container app.')
param location string

@description('Name of the Container Apps environment.')
param environmentName string

@description('Name of the container app.')
param containerAppName string

@description('Resource ID of the VNet infrastructure subnet (from networking.bicep).')
param infraSubnetId string

@description('Docker Hub image reference, e.g. docker.io/bralmeida/smtprelay:latest.')
param containerImage string

@description('CPU cores allocated to the Consumption workload profile replica.')
param cpu string = '0.25'

@description('Memory allocated to the Consumption workload profile replica.')
param memory string = '0.5Gi'

@minValue(0)
param minReplicas int = 0

@minValue(1)
param maxReplicas int = 1

@description('UOL upstream SMTP host. Confirmed via real test (docs/dns.md): smtps.uol.com.br:587 with STARTTLS is correct.')
param smtpUpstreamHost string = 'smtps.uol.com.br'

param smtpUpstreamPort string = '587'

@description('Public hostname clients point their mail app at (custom domain binding is configured separately -- see docs/dns.md).')
param relayDomain string = 'smtps.bratech.me'

param maxMessageSizeBytes string = '15728640'
param maxRecipients string = '20'
param readTimeout string = '30s'
param writeTimeout string = '60s'
param upstreamTimeout string = '30s'
param rateLimitConnPerMinute string = '30'
param rateLimitMsgsPerHour string = '200'

@secure()
@description('The single fixed relay account -- also the real UOL mailbox credentials.')
param relayUser string

@secure()
param relayPassword string

@secure()
@description('PEM-encoded TLS certificate (client-facing, for relayDomain), base64 is NOT required -- pass the raw PEM text.')
param tlsCertPem string

@secure()
@description('PEM-encoded TLS private key matching tlsCertPem.')
param tlsKeyPem string

resource environment 'Microsoft.App/managedEnvironments@2024-03-01' = {
  name: environmentName
  location: location
  properties: {
    vnetConfiguration: {
      infrastructureSubnetId: infraSubnetId
    }
    workloadProfiles: [
      {
        name: 'Consumption'
        workloadProfileType: 'Consumption'
      }
    ]
  }
}

resource containerApp 'Microsoft.App/containerApps@2024-03-01' = {
  name: containerAppName
  location: location
  properties: {
    managedEnvironmentId: environment.id
    workloadProfileName: 'Consumption'
    configuration: {
      activeRevisionsMode: 'Single'
      ingress: {
        external: true
        transport: 'tcp'
        targetPort: 587
        exposedPort: 587
        additionalPortMappings: [
          {
            external: true
            targetPort: 465
            exposedPort: 465
          }
        ]
      }
      secrets: [
        { name: 'relay-user', value: relayUser }
        { name: 'relay-password', value: relayPassword }
        { name: 'tls-cert', value: tlsCertPem }
        { name: 'tls-key', value: tlsKeyPem }
      ]
    }
    template: {
      containers: [
        {
          name: 'smtprelay'
          image: containerImage
          resources: {
            cpu: json(cpu)
            memory: memory
          }
          env: [
            { name: 'RELAY_USER', secretRef: 'relay-user' }
            { name: 'RELAY_PASSWORD', secretRef: 'relay-password' }
            { name: 'RELAY_DOMAIN', value: relayDomain }
            { name: 'TLS_CERT_FILE', value: '/etc/smtprelay/tls/tls.crt' }
            { name: 'TLS_KEY_FILE', value: '/etc/smtprelay/tls/tls.key' }
            { name: 'SMTP_UPSTREAM_HOST', value: smtpUpstreamHost }
            { name: 'SMTP_UPSTREAM_PORT', value: smtpUpstreamPort }
            { name: 'LISTEN_ADDR_STARTTLS', value: ':587' }
            { name: 'LISTEN_ADDR_IMPLICIT_TLS', value: ':465' }
            { name: 'HEALTH_ADDR', value: ':8080' }
            { name: 'MAX_MESSAGE_SIZE_BYTES', value: maxMessageSizeBytes }
            { name: 'MAX_RECIPIENTS', value: maxRecipients }
            { name: 'READ_TIMEOUT', value: readTimeout }
            { name: 'WRITE_TIMEOUT', value: writeTimeout }
            { name: 'UPSTREAM_TIMEOUT', value: upstreamTimeout }
            { name: 'RATE_LIMIT_CONN_PER_MINUTE', value: rateLimitConnPerMinute }
            { name: 'RATE_LIMIT_MSGS_PER_HOUR', value: rateLimitMsgsPerHour }
          ]
          volumeMounts: [
            {
              volumeName: 'tls-certs'
              mountPath: '/etc/smtprelay/tls'
            }
          ]
          probes: [
            {
              type: 'Liveness'
              httpGet: { path: '/healthz', port: 8080 }
              initialDelaySeconds: 5
              periodSeconds: 30
            }
            {
              // Readiness only gates traffic; it must never feed a restart
              // policy, so a temporary UOL outage doesn't cause a restart
              // loop (see docs/operations.md).
              type: 'Readiness'
              httpGet: { path: '/readyz', port: 8080 }
              initialDelaySeconds: 5
              periodSeconds: 15
            }
          ]
        }
      ]
      volumes: [
        {
          name: 'tls-certs'
          storageType: 'Secret'
          secrets: [
            { secretRef: 'tls-cert', path: 'tls.crt' }
            { secretRef: 'tls-key', path: 'tls.key' }
          ]
        }
      ]
      scale: {
        minReplicas: minReplicas
        maxReplicas: maxReplicas
      }
    }
  }
}

output containerAppFqdn string = containerApp.properties.configuration.ingress.fqdn
output environmentId string = environment.id
