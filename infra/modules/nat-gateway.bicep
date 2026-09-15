// Modo B only: a NAT Gateway on the infra subnet gives the relay a single,
// stable, dedicated outbound IP for the connection to UOL -- instead of the
// Standard Load Balancer's rotating shared SNAT pool. This is the only
// piece of infrastructure that costs real, ongoing money (gateway hours +
// data processed); see ../../docs/cost.md. Deployed only when
// enableDedicatedEgress = true (see ../main.bicep).

@description('Region for the NAT Gateway and its public IP.')
param location string

@description('Name of the NAT Gateway.')
param natGatewayName string

@description('Name of the dedicated outbound public IP.')
param publicIpName string

@description('Name of the VNet the infra subnet belongs to.')
param vnetName string

@description('Name of the infra subnet to attach the NAT Gateway to.')
param infraSubnetName string

resource publicIp 'Microsoft.Network/publicIPAddresses@2023-11-01' = {
  name: publicIpName
  location: location
  sku: {
    name: 'Standard'
  }
  properties: {
    publicIPAllocationMethod: 'Static'
  }
}

resource natGateway 'Microsoft.Network/natGateways@2023-11-01' = {
  name: natGatewayName
  location: location
  sku: {
    name: 'Standard'
  }
  properties: {
    publicIpAddresses: [
      {
        id: publicIp.id
      }
    ]
  }
}

// Associates the NAT Gateway with the infra subnet created in
// networking.bicep. Using a separate subnet resource (same name, same
// parent VNet) rather than a circular module reference back into
// networking.bicep.
resource vnet 'Microsoft.Network/virtualNetworks@2023-11-01' existing = {
  name: vnetName
}

resource infraSubnet 'Microsoft.Network/virtualNetworks/subnets@2023-11-01' = {
  parent: vnet
  name: infraSubnetName
  properties: {
    addressPrefix: vnet.properties.subnets[0].properties.addressPrefix
    // This resource redeclares the whole subnet (Bicep/ARM PUT semantics),
    // so the delegation from networking.bicep has to be repeated here too
    // -- otherwise attaching the NAT Gateway silently wipes it out.
    delegations: vnet.properties.subnets[0].properties.delegations
    natGateway: {
      id: natGateway.id
    }
  }
}

output natGatewayId string = natGateway.id
output dedicatedEgressIp string = publicIp.properties.ipAddress
