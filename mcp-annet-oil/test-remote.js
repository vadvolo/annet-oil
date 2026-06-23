#!/usr/bin/env node

import { AnnetOilClient } from './dist/client.js';

const client = new AnnetOilClient({
  apiUrl: 'http://192.168.52.235:8181',
  authToken: 'rYdddlPWkrYdddlPWk',
  timeout: 30000,
});

async function runTests() {
  console.log('Testing Annet Oil API at http://192.168.52.235:8181');
  console.log('='*60);

  try {
    // Test 1: Health check
    console.log('\n1. Testing health endpoint...');
    const health = await client.health();
    console.log('✓ Health check:', health);
  } catch (error) {
    console.error('✗ Health check failed:', error.message);
  }

  try {
    // Test 2: Get containers
    console.log('\n2. Testing container status...');
    const containers = await client.getContainers();
    console.log('✓ Containers:', JSON.stringify(containers, null, 2));
  } catch (error) {
    console.error('✗ Container status failed:', error.message);
  }

  try {
    // Test 3: Generate config for a device
    console.log('\n3. Testing gen command for Kragujevac-4948-10G.otk.rs...');
    const genResult = await client.gen({
      filters: ['Kragujevac-4948-10G.otk.rs'],
    });
    console.log('✓ Gen result:');
    console.log(`  Success: ${genResult.success}`);
    console.log(`  Total hosts: ${genResult.total_hosts}`);
    console.log(`  Success hosts: ${genResult.success_hosts}`);
    console.log(`  Failed hosts: ${genResult.failed_hosts}`);

    if (genResult.results) {
      for (const [hostname, result] of Object.entries(genResult.results)) {
        console.log(`\n  Device: ${hostname}`);
        console.log(`    Container: ${result.container}`);
        console.log(`    Exit code: ${result.exit_code}`);
        if (result.stdout) {
          console.log(`    Config length: ${result.stdout.length} chars`);
          console.log(`    First 200 chars: ${result.stdout.substring(0, 200)}...`);
        }
        if (result.error) {
          console.log(`    Error: ${result.error}`);
        }
      }
    }
  } catch (error) {
    console.error('✗ Gen command failed:', error.message);
  }

  try {
    // Test 4: Show diff for a device
    console.log('\n4. Testing diff command for Kragujevac-4948-10G.otk.rs...');
    const diffResult = await client.diff({
      filters: ['Kragujevac-4948-10G.otk.rs'],
    });
    console.log('✓ Diff result:');
    console.log(`  Success: ${diffResult.success}`);

    if (diffResult.results) {
      for (const [hostname, result] of Object.entries(diffResult.results)) {
        console.log(`\n  Device: ${hostname}`);
        console.log(`    Exit code: ${result.exit_code}`);
        if (result.stdout) {
          console.log(`    Diff output: ${result.stdout.substring(0, 500)}...`);
        }
      }
    }
  } catch (error) {
    console.error('✗ Diff command failed:', error.message);
  }

  try {
    // Test 5: Get routing info
    console.log('\n5. Testing routing information...');
    const routing = await client.getRouting('Kragujevac-4948-10G.otk.rs');
    console.log('✓ Routing:', JSON.stringify(routing, null, 2));
  } catch (error) {
    console.error('✗ Routing query failed:', error.message);
  }

  console.log('\n' + '='*60);
  console.log('Testing complete!');
}

runTests().catch(console.error);