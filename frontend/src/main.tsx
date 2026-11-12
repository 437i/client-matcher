import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'

import App from '@/App'
import '@/index.css'

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      // Повтор запроса один раз помогает при микросбоях сети, но не должен
      // превращаться в долгое ожидание для сотрудника.
      retry: 1,
      refetchOnWindowFocus: false,
    },
    mutations: {
      // Создание связи повторять нельзя: второй запрос может создать дубль.
      retry: false,
    },
  },
})

const container = document.getElementById('root')
if (!container) {
  throw new Error('Не найден корневой элемент #root')
}

createRoot(container).render(
  <StrictMode>
    <QueryClientProvider client={queryClient}>
      <App />
    </QueryClientProvider>
  </StrictMode>,
)
