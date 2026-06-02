import { onBeforeUnmount, ref, watch, type Ref } from 'vue'

interface AuthenticatedImageOptions {
  sourceUrl: Ref<string>
  getToken: () => string
}

export const useAuthenticatedImage = ({ sourceUrl, getToken }: AuthenticatedImageOptions) => {
  const imageUrl = ref('')
  const loading = ref(false)
  let objectUrl = ''
  let requestController: AbortController | null = null

  const releaseObjectUrl = () => {
    if (!objectUrl) return
    URL.revokeObjectURL(objectUrl)
    objectUrl = ''
  }

  watch(sourceUrl, async nextUrl => {
    requestController?.abort()
    releaseObjectUrl()
    imageUrl.value = ''
    if (!nextUrl) return
    if (nextUrl.startsWith('data:') || nextUrl.startsWith('blob:')) {
      imageUrl.value = nextUrl
      return
    }

    const controller = new AbortController()
    requestController = controller
    loading.value = true
    try {
      const response = await fetch(nextUrl, {
        headers: { Authorization: getToken() },
        signal: controller.signal
      })
      if (!response.ok) return
      objectUrl = URL.createObjectURL(await response.blob())
      imageUrl.value = objectUrl
    } catch (error) {
      if ((error as Error).name !== 'AbortError') {
        imageUrl.value = ''
      }
    } finally {
      if (requestController === controller) loading.value = false
    }
  }, { immediate: true })

  onBeforeUnmount(() => {
    requestController?.abort()
    releaseObjectUrl()
  })

  return { imageUrl, loading }
}
