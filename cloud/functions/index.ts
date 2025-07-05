import * as functions from '@google-cloud/functions-framework';
import * as compute from '@google-cloud/compute'

const PROJECT = process.env.GCP_PROJECT || 'bylcraft';
const ZONE = process.env.GCP_ZONE || "europe-west3-a";

function delay(ms: number) {
  return new Promise(resolve => setTimeout(resolve, ms));
}

export const startServer = functions.http('startServer', async (req, res) => {
  const client = new compute.InstancesClient();

  const response = await client.start({
    project: PROJECT,
    zone: ZONE,
    instance: 'minecraft-server'
  });

  console.log(`Response from starting instance: ${JSON.stringify(response)}`);

  var statusResponse = await client.get({
    project: PROJECT,
    zone: ZONE,
    instance: 'minecraft-server'
  });
  while(statusResponse?.[0]?.status !== 'RUNNING') {
    console.log(`Instance status: ${statusResponse?.[0]?.status}`);
    await delay(5000); // Wait for 5 seconds before checking again
    statusResponse = await client.get({
      project: PROJECT,
      zone: ZONE,
      instance: 'minecraft-server'
    });
  }

  res.send(`Der Minecraft Server ist gestartet! Er ist jetzt erreichbar unter: ${statusResponse?.[0]?.networkInterfaces?.[0]?.accessConfigs?.[0]?.natIP || 'unbekannt'}:25565`);
});

export const stopServer = functions.http('stopServer', async (req, res) => {
  const client = new compute.InstancesClient();

  const response = await client.stop({
    project: PROJECT,
    zone: ZONE,
    instance: 'minecraft-server'
  });

  console.log(`Response from stopping instance: ${JSON.stringify(response)}`);

  var statusResponse = await client.get({
    project: PROJECT,
    zone: ZONE,
    instance: 'minecraft-server'
  });
  while(statusResponse?.[0]?.status !== 'TERMINATED') {
    console.log(`Instance status: ${statusResponse?.[0]?.status}`);
    await delay(5000); // Wait for 5 seconds before checking again
    statusResponse = await client.get({
      project: PROJECT,
      zone: ZONE,
      instance: 'minecraft-server'
    });
  }

  res.send(`Der Minecraft Server ist jetzt gestoppt! Der Status ist: ${statusResponse?.[0]?.status}`);
});
