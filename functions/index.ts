import * as functions from '@google-cloud/functions-framework';
import * as compute from '@google-cloud/compute'

const PROJECT = process.env.GCP_PROJECT || 'bylcraft';
const ZONE = process.env.GCP_ZONE || "europe-west3-a";

export const helloHttp = functions.http('startServer', async (req, res) => {
  const client = new compute.InstancesClient();

  const response = await client.start({
    project: PROJECT,
    zone: ZONE,
    instance: 'minecraft-server'
  })

  res.send(`Hello ${req.query.name || req.body.name || 'World, I am BylCraft with'} ${JSON.stringify(response)}!`);
});
