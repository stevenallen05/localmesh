import type { NextApiRequest, NextApiResponse } from 'next';
import * as grpc from '@grpc/grpc-js';
import * as protoLoader from '@grpc/proto-loader';
import { promisify } from 'util';
import path from 'path';
import { logger } from '../../lib/logger';

const PROTO_PATH = path.resolve(process.cwd(), 'proto/metrics.proto');

const packageDefinition = protoLoader.loadSync(PROTO_PATH, {
  keepCase: true,
  longs: String,
  enums: String,
  defaults: true,
  oneofs: true,
});
const proto = grpc.loadPackageDefinition(packageDefinition) as any;
const CatalogCtor = proto.metrics.v1.Catalog;

type DataSource = { uid: string; name: string; type: string; url: string };

export default async function handler(req: NextApiRequest, res: NextApiResponse) {
  if (req.method !== 'POST') {
    return res.status(405).json({ error: 'Method not allowed' });
  }

  const target = process.env.SERVER_ADDR ?? 'server:50051';
  const client = new CatalogCtor(target, grpc.credentials.createInsecure());
  const listDataSources = promisify(client.ListDataSources.bind(client)) as
    (req: object) => Promise<{ data_sources: DataSource[] }>;
  const listMetrics = promisify(client.ListMetrics.bind(client)) as
    (req: { datasource_uid: string }) => Promise<{ names: string[] }>;

  try {
    const { data_sources: dataSources } = await listDataSources({});
    const result = await Promise.all(dataSources.map(async (ds) => ({
      ...ds,
      metrics: (await listMetrics({ datasource_uid: ds.uid })).names ?? [],
    })));
    logger.info({ catalog: result }, 'metrics catalog');
    res.status(200).json({ dataSources: result });
  } catch (err: unknown) {
    const message = err instanceof Error ? err.message : 'unknown';
    logger.error({ err: message }, 'list-metrics-catalog failed');
    res.status(500).json({ error: 'catalog call failed', message });
  } finally {
    client.close();
  }
}
