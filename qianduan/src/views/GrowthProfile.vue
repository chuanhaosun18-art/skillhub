<!--
  看别人的成长身份。只显示对方公开的部分——未公开的段落会明确写「未公开」，
  而不是假装那里什么都没有。这是「前路关系」的落点：我连接你，不是因为兴趣相同，
  而是因为你走过我正在走的路。
-->
<template>
  <div class="page">
    <AppNavbar />
    <main class="main">
      <div class="head">
        <h1>成长身份</h1>
        <p class="sub">
          这里没有粉丝数，也没有排名。你看到的是这个人真实走过的路，
          以及他沉淀下来的、你可以直接用的方法。
        </p>
      </div>
      <section class="card">
        <GrowthPath :user-id="userId" />
      </section>
    </main>
  </div>
</template>

<script setup>
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import AppNavbar from '../components/AppNavbar.vue'
import GrowthPath from '../components/GrowthPath.vue'

const route = useRoute()
const userId = computed(() => route.params.id)
</script>

<style scoped>
.page {
  min-height: 100vh;
  background: #f5f7fa;
}
.main {
  max-width: 860px;
  margin: 0 auto;
  padding: 28px 20px 60px;
}
.head h1 {
  margin: 0 0 6px;
  font-size: 24px;
}
.sub {
  color: #909399;
  font-size: 13px;
  line-height: 1.8;
  margin: 0 0 18px;
  max-width: 620px;
}
.card {
  background: #fff;
  border-radius: 10px;
  padding: 24px;
  box-shadow: 0 2px 10px rgba(0, 0, 0, 0.04);
}
</style>
