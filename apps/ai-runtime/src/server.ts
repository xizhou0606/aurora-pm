import { buildApp } from './app.js';
import { loadConfig } from './config.js';
import { createGenerator } from './capabilities/registry.js';

const config = loadConfig();
const app = buildApp(config, createGenerator(config));

await app.listen({ host: config.AI_HTTP_ADDR, port: config.AI_HTTP_PORT });
