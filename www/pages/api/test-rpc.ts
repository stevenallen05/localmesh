import type { NextApiRequest, NextApiResponse } from 'next';
import * as grpc from '@grpc/grpc-js';
import * as protoLoader from '@grpc/proto-loader';
import { promisify } from 'util';
import path from 'path';

// Get the proto file path relative to the project root
// process.cwd() in Next.js API routes is the www directory, so we need to go up one level
const PROTO_PATH = path.resolve(process.cwd(), '../proto/test/test.proto');

const packageDefinition = protoLoader.loadSync(PROTO_PATH, {
  keepCase: true,
  longs: String,
  enums: String,
  defaults: true,
  oneofs: true,
});

const { test } = grpc.loadPackageDefinition(packageDefinition) as any;

type TestClient = {
  testMethod: (
    request: { test_before: string },
    callback: (error: any, response: { test_after: string }) => void
  ) => void;
};

export default async function handler(
  req: NextApiRequest,
  res: NextApiResponse
) {
  if (req.method !== 'POST') {
    return res.status(405).json({ error: 'Method not allowed' });
  }

  const { test_before } = req.body;

  if (!test_before) {
    return res.status(400).json({ error: 'test_before is required' });
  }

  try {
    const client = new test.Test(
      '127.0.0.1:3001',
      grpc.credentials.createInsecure()
    ) as TestClient;

    const testMethod = promisify(client.testMethod.bind(client));

    const response = await testMethod({ test_before });

    res.status(200).json(response);
  } catch (error: any) {
    console.error('gRPC error:', error);
    res.status(500).json({ 
      error: 'Failed to call gRPC service',
      message: error.message 
    });
  }
}

