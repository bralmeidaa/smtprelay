// VNet + infrastructure subnet for the Container Apps environment.
//
// Mandatory in BOTH modes: Microsoft Learn's ingress docs are explicit that
// "External TCP ingress is only supported for Container Apps environments
// that use a virtual network" -- SMTP ingress on 587/465 is raw TCP, so a
// VNet-integrated (workload profiles) environment is a hard requirement
// even in the "economic" Modo A, not just an add-on for Modo B's NAT
// Gateway. See ../../docs/architecture.md for the full explanation.

@description('Region for the VNet and subnet.')
param location string

@description('Name of the virtual network.')
param vnetName string

@description('Address space of the virtual network.')
param vnetAddressPrefix string = '10.20.0.0/23'

@description('Address prefix of the Container Apps environment infrastructure subnet.')
param infraSubnetAddressPrefix string = '10.20.0.0/24'

resource vnet 'Microsoft.Network/virtualNetworks@2023-11-01' = {
  name: vnetName
  location: location
  properties: {
    addressSpace: {
      addressPrefixes: [vnetAddressPrefix]
    }
    subnets: [
      {
        name: 'infra'
        properties: {
          addressPrefix: infraSubnetAddressPrefix
        }
      }
    ]
  }
}

output vnetId string = vnet.id
output infraSubnetId string = vnet.properties.subnets[0].id
output infraSubnetName string = vnet.properties.subnets[0].name
