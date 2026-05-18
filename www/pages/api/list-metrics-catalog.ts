import type { NextApiRequest, NextApiResponse } from 'next';
import * as grpc from '@grpc/grpc-js';
import * as protoLoader from '@grpc/proto-loader';
import path from 'path';
import { trace } from '@opentelemetry/api';
import { logger } from '../../lib/logger';
import { meshChannelCredentials } from '../../lib/grpc-credentials';
import { getUser, userMetadata } from '../../lib/identity';

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

  // PII-at-ingress: see lib/identity.ts header.
  const user = getUser(req);
  trace.getActiveSpan()?.setAttributes({
    'enduser.id':    user.id,
    'enduser.email': user.email,
    'user.id':       user.id,
    'user.email':    user.email,
  });
  const md = userMetadata(user);

  const target = process.env.SERVER_ADDR ?? 'server:50051';
  const client = new CatalogCtor(target, meshChannelCredentials());
  const listDataSources = (): Promise<{ data_sources: DataSource[] }> =>
    new Promise((resolve, reject) =>
      client.ListDataSources({}, md, (err: unknown, r: { data_sources: DataSource[] }) =>
        err ? reject(err) : resolve(r),
      ),
    );
  const listMetrics = (datasource_uid: string): Promise<{ names: string[] }> =>
    new Promise((resolve, reject) =>
      client.ListMetrics({ datasource_uid }, md, (err: unknown, r: { names: string[] }) =>
        err ? reject(err) : resolve(r),
      ),
    );

  try {
    const { data_sources: dataSources } = await listDataSources();
    const result = await Promise.all(dataSources.map(async (ds) => ({
      ...ds,
      metrics: (await listMetrics(ds.uid)).names ?? [],
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
