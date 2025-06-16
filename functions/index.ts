import * as functions from '@google-cloud/functions-framework';

export const helloHttp = functions.http('helloHttp', (req, res) => {
  res.send(`Hello ${req.query.name || req.body.name || 'World, I am BylCraft'}!`);
});
