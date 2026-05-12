import "server-only";
import * as grpc from "@grpc/grpc-js";
import * as protoLoader from "@grpc/proto-loader";
import path from "path";

// Render on every request — never prerender at build time. The gRPC server
// isn't available during `next build`, and the response shouldn't be cached.
export const dynamic = "force-dynamic";

// process.cwd() == /app/www inside the container (Dockerfile WORKDIR) and the
// www/ project root locally. `../proto/hello.proto` works for both.
const PROTO_PATH = path.resolve(process.cwd(), "../proto/hello.proto");

const pkgDef = protoLoader.loadSync(PROTO_PATH, {
  keepCase: true,
  longs: String,
  enums: String,
  defaults: true,
  oneofs: true,
});
const pkg = grpc.loadPackageDefinition(pkgDef) as any;

// TODO: per-request channel for dev simplicity. Prod swaps to a real LB
// (envoy/grpc-web or service mesh) and hoists the client into a module-level
// singleton; see DESIGN_DECISIONS.md "gRPC LB".
async function sayHello(name: string): Promise<string> {
  const addr = process.env.SERVER_ADDR ?? "127.0.0.1:50051";
  const client = new pkg.hello.Greeter(addr, grpc.credentials.createInsecure());
  return new Promise((resolve, reject) => {
    client.SayHello({ name }, (err: grpc.ServiceError | null, res: { message: string }) => {
      if (err) reject(err);
      else resolve(res.message);
    });
  });
}

export default async function Home() {
  const message = await sayHello("NextJS");
  return <main>{message}</main>;
}
