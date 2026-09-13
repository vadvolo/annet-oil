#!/usr/bin/env node

import { AnnetOilClient } from './dist/client.js';

const client = new AnnetOilClient({
  apiUrl: 'http://192.168.52.235:8181',
  authToken: 'YOUR_API_TOKEN',
  timeout: 30000,
});

async function testExecuteWithHost() {
  console.log('Testing Annet Oil execute with host parameter');
  console.log('='*60);

  try {
    // Test with host parameter directly
    console.log('\nTesting execute with host as filters...');
    const request = {
      command: 'show version',
      filters: ['192.168.29.55'],  // Using IP as first filter element
    };

    console.log('Request:', JSON.stringify(request, null, 2));
    const response = await client.executeCommand(request);
    console.log('✓ Execute succeeded!');
    console.log('Response:', JSON.stringify(response, null, 2));
  } catch (error) {
    console.error('✗ Execute failed:', error.message);
  }
}

testExecuteWithHost().catch(console.error);