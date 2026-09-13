#!/usr/bin/env node

import { CommandValidator } from './dist/command-whitelist.js';

const validator = new CommandValidator();

console.log('Available command categories:');
console.log('='*60);
const categories = validator.getCategories();
categories.forEach((cat, i) => {
  console.log(`${i+1}. ${cat}`);
});

console.log('\nTesting new commands:');
console.log('='*60);

const testCommands = [
  'show ip bgp summary',
  'show ip bgp neighbors',
  'show ip bgp neighbors 192.168.1.1',
  'show ip bgp neighbors 192.168.1.1 routes',
  'show ip bgp neighbors 192.168.1.1 advertised-routes',
  'show ip route bgp',
  'show interfaces eth0',
  'show logging',
  'show route-map',
  'show route-map TEST-MAP',
  'show ip access-lists',
  'show ip access-lists ACL-100',
  'ping 8.8.8.8',
];

testCommands.forEach(cmd => {
  const isAllowed = validator.isAllowed(cmd);
  console.log(`${isAllowed ? '✓' : '✗'} ${cmd}`);
});

console.log('\nTesting autocomplete suggestions:');
console.log('='*60);
const partials = ['show ip b', 'show route', 'ping'];
partials.forEach(partial => {
  const suggestions = validator.getSuggestions(partial);
  console.log(`\n"${partial}":`);
  suggestions.forEach(s => console.log(`  - ${s}`));
});