#!/usr/bin/env node
/**
 * Test script for MCP server with filtered generator support
 */

const { AnnetOilClient } = require('./dist/client.js');

const API_URL = process.env.ANNET_OIL_API_URL || 'http://localhost:8080';
const AUTH_TOKEN = process.env.ANNET_OIL_AUTH_TOKEN || 'change-me-in-production';

const client = new AnnetOilClient({
  apiUrl: API_URL,
  authToken: AUTH_TOKEN,
  timeout: 30000,
});

async function testFilteredGenerators() {
  console.log('Testing MCP Annet Oil Client with Filtered Generators');
  console.log(`API URL: ${API_URL}\n`);

  try {
    // Test 1: Gen with specific generator
    console.log('Test 1: Generate with specific generator (description)');
    console.log('=' .repeat(60));
    const test1 = await client.gen({
      filters: ['router1.example.com'],
      generators: ['description'],
      dry_run: true,
    });
    console.log('Result:', JSON.stringify(test1, null, 2));
    console.log('\n');

    // Test 2: Gen with exclude generator
    console.log('Test 2: Generate excluding specific generator (hostname)');
    console.log('=' .repeat(60));
    const test2 = await client.gen({
      filters: ['router1.example.com'],
      exclude_generators: ['hostname'],
      dry_run: true,
    });
    console.log('Result:', JSON.stringify(test2, null, 2));
    console.log('\n');

    // Test 3: Gen with both include and exclude
    console.log('Test 3: Generate with both include and exclude generators');
    console.log('=' .repeat(60));
    const test3 = await client.gen({
      filters: ['router1.example.com'],
      generators: ['interfaces', 'routing'],
      exclude_generators: ['acl'],
      dry_run: true,
    });
    console.log('Result:', JSON.stringify(test3, null, 2));
    console.log('\n');

    // Test 4: Diff with generator filter
    console.log('Test 4: Diff with generator filter');
    console.log('=' .repeat(60));
    const test4 = await client.diff({
      filters: ['switch1.example.com'],
      generators: ['vlans'],
      dry_run: true,
    });
    console.log('Result:', JSON.stringify(test4, null, 2));
    console.log('\n');

    // Test 5: Multiple devices with generator filter
    console.log('Test 5: Multiple devices with generator filter');
    console.log('=' .repeat(60));
    const test5 = await client.gen({
      filters: ['router1.example.com', 'router2.example.com'],
      generators: ['interfaces'],
      dry_run: true,
    });
    console.log('Result:', JSON.stringify(test5, null, 2));
    console.log('\n');

    // Test 6: Test with patch command
    console.log('Test 6: Patch with generator filter');
    console.log('=' .repeat(60));
    const test6 = await client.patch({
      filters: ['router1.example.com'],
      generators: ['interfaces'],
      dry_run: true,
    });
    console.log('Result:', JSON.stringify(test6, null, 2));
    console.log('\n');

  } catch (error) {
    console.error('Test failed:', error.message);
    if (error.response) {
      console.error('Response data:', error.response.data);
    }
  }
}

// Check container status
async function checkContainers() {
  console.log('Checking container status...');
  console.log('=' .repeat(60));
  try {
    const containers = await client.getContainers();
    console.log('Containers:', JSON.stringify(containers, null, 2));
  } catch (error) {
    console.error('Failed to get containers:', error.message);
  }
}

// Main function
async function main() {
  await checkContainers();
  console.log('\n');
  await testFilteredGenerators();
}

main().catch(error => {
  console.error('Fatal error:', error);
  process.exit(1);
});