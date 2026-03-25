# Outbound Performance Testing Guide

## Pre-Test Setup
1. Ensure you have the 3x-ui panel running
2. Backup your current database before testing
3. Use a test environment, not production

## Test Scenarios

### Scenario 1: Small Dataset (Baseline)
**Steps:**
1. Create 50 outbound configurations
2. Navigate to Xray Configs → Outbounds
3. Test pagination controls
4. Test search functionality

**Expected Results:**
- Page loads instantly
- Pagination shows "Total 50 items"
- Can navigate between pages
- Search filters correctly

### Scenario 2: Medium Dataset (500 outbounds)
**Steps:**
1. Create/import 500 outbound configurations
2. Navigate to Xray Configs → Outbounds
3. Verify pagination shows 10 pages (50 per page)
4. Test page size selector (20, 50, 100, 200, 500)
5. Test search for specific tags
6. Test add/edit/delete operations

**Expected Results:**
- Page loads quickly (< 1 second)
- Pagination controls work smoothly
- Changing page size updates view instantly
- Search filters work correctly
- No browser lag or freezing

### Scenario 3: Large Dataset (1400+ outbounds) - Critical Test
**Steps:**
1. Create/import 1400+ outbound configurations
2. Navigate to Xray Configs → Outbounds
3. Measure page load time
4. Test scrolling through pages
5. Test search functionality
6. Open edit dialog for an outbound
7. Test traffic refresh button
8. Wait for WebSocket traffic updates

**Expected Results:**
- Page loads in < 2 seconds
- Smooth scrolling between pages
- Search works instantly
- Edit dialog opens quickly
- No browser freezing or "page not responding" warnings
- Traffic updates don't cause UI lag
- All controls remain responsive

### Scenario 4: Search Functionality
**Steps:**
1. With 1000+ outbounds, test various searches:
   - Search by exact tag name
   - Search by partial tag name
   - Search by protocol (e.g., "vmess", "vless")
   - Clear search and verify all items shown

**Expected Results:**
- Search results appear instantly
- Pagination updates to show correct number of results
- Clearing search returns to full list

### Scenario 5: Edge Cases
**Steps:**
1. Delete all items on last page - verify page adjusts
2. Add item while on page 10 - verify moves to page 1
3. Search, then add item - verify behavior
4. Search, then delete item - verify behavior

**Expected Results:**
- Pagination adjusts correctly
- No crashes or errors
- UI remains responsive

## Performance Metrics

### Before Fix (1400+ outbounds):
- Page load: 10-30 seconds
- Scroll lag: Severe
- Browser response: Often freezes
- Memory usage: Very high

### After Fix (Expected):
- Page load: < 2 seconds
- Scroll lag: None
- Browser response: Smooth
- Memory usage: Normal

## Browser DevTools Monitoring

### Check Performance:
1. Open Chrome DevTools (F12)
2. Go to Performance tab
3. Start recording
4. Navigate to Outbounds page
5. Stop recording
6. Analyze:
   - Frame rate (should stay near 60fps)
   - Layout/paint times (should be minimal)
   - Script execution time

### Check Memory:
1. Open Chrome DevTools (F12)
2. Go to Memory tab
3. Take heap snapshot before/after
4. Compare:
   - Number of DOM nodes (should be ~50x page elements)
   - Memory usage (should be much lower)

### Check Network:
1. Open Chrome DevTools (F12)
2. Go to Network tab
3. Monitor WebSocket messages
4. Verify traffic updates don't cause main thread blocking

## Test Data Generation

To generate test outbounds, you can use this script:

```javascript
// Run in browser console on the Outbounds page
const protocols = ['vmess', 'vless', 'trojan', 'shadowsocks'];
for (let i = 0; i < 1400; i++) {
    const protocol = protocols[i % protocols.length];
    const tag = `test-${protocol}-${i}`;
    // Add your outbound creation logic here
}
```

## Reporting Results

If you encounter issues, please report:
1. Number of outbounds
2. Browser name and version
3. Page load time
4. Specific actions that cause problems
5. Browser console errors (if any)
6. Performance profiler screenshots (if possible)

## Success Criteria
✅ Page loads quickly even with 1400+ outbounds
✅ No browser freezing or "not responding" warnings
✅ All controls remain responsive
✅ Search works correctly
✅ Pagination works smoothly
✅ Add/edit/delete operations maintain correct state
✅ WebSocket traffic updates don't cause UI lag
