<template>
  <div class="page dashboard">
    <header class="topbar">
      <h2>Dashboard</h2>
      <span class="date">{{ new Date().toLocaleDateString() }}</span>
    </header>

    <div class="stats">
      <div class="stat-card">
        <span class="stat-label">Total Users</span>
        <span class="stat-num">1,284</span>
        <span class="stat-change up">+12%</span>
      </div>
      <div class="stat-card">
        <span class="stat-label">Active Orders</span>
        <span class="stat-num">342</span>
        <span class="stat-change up">+5%</span>
      </div>
      <div class="stat-card">
        <span class="stat-label">Products</span>
        <span class="stat-num">18</span>
        <span class="stat-change">—</span>
      </div>
      <div class="stat-card">
        <span class="stat-label">Revenue (MTD)</span>
        <span class="stat-num">$12,480</span>
        <span class="stat-change up">+8%</span>
      </div>
    </div>

    <div class="chart-section">
      <h3>Recent Orders</h3>
      <table class="data-table">
        <thead><tr><th>ID</th><th>Customer</th><th>Product</th><th>Amount</th><th>Status</th></tr></thead>
        <tbody>
          <tr><td>#1024</td><td>john.doe</td><td>Pro VPN</td><td>$19.99</td><td><span class="status active">Active</span></td></tr>
          <tr><td>#1023</td><td>jane.smith</td><td>Starter VPN</td><td>$9.99</td><td><span class="status pending">Pending</span></td></tr>
          <tr><td>#1022</td><td>bob.wilson</td><td>Proxy Pack M</td><td>$39.99</td><td><span class="status active">Active</span></td></tr>
          <tr><td>#1021</td><td>alice.j</td><td>Dedicated IP</td><td>$4.99</td><td><span class="status expired">Expired</span></td></tr>
        </tbody>
      </table>
    </div>
  </div>
</template>

<script setup lang="ts">
</script>

<style scoped>
.stats {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 1rem;
  padding: 1.5rem 2rem;
}
.stat-card {
  background: #1a1d23;
  border-radius: 10px;
  padding: 1.25rem;
  border: 1px solid #2a2d35;
}
.stat-label { display: block; font-size: 0.8rem; color: #888; text-transform: uppercase; letter-spacing: 0.5px; margin-bottom: 0.5rem; }
.stat-num { display: block; font-size: 1.75rem; font-weight: 700; color: #fff; }
.stat-change { display: inline-block; font-size: 0.75rem; margin-top: 0.3rem; }
.stat-change.up { color: #4caf50; }
.chart-section {
  padding: 0 2rem 2rem;
}
.chart-section h3 { color: #fff; font-size: 1rem; margin: 0 0 1rem; }
.data-table {
  width: 100%;
  border-collapse: collapse;
  background: #1a1d23;
  border-radius: 10px;
  overflow: hidden;
}
.data-table th, .data-table td {
  text-align: left;
  padding: 0.75rem 1rem;
  border-bottom: 1px solid #2a2d35;
  font-size: 0.9rem;
}
.data-table th { color: #888; font-weight: 600; text-transform: uppercase; font-size: 0.75rem; letter-spacing: 0.5px; }
.data-table td { color: #ccc; }
.status {
  padding: 0.2rem 0.5rem;
  border-radius: 4px;
  font-size: 0.75rem;
  font-weight: 600;
}
.status.active { background: #1a3a1a; color: #4caf50; }
.status.pending { background: #3a3a1a; color: #ffc107; }
.status.expired { background: #3a1a1a; color: #ff6b6b; }
</style>
