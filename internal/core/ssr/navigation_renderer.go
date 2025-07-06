package ssr

import (
	"fmt"
	"strings"
)

type NavigationItem struct {
	Title    string
	URL      string
	Children []NavigationItem
}

type NavigationRenderer struct {
}

func NewNavigationRenderer() *NavigationRenderer {
	return &NavigationRenderer{}
}

/**

<div class="flex flex-1 flex-col">
   <div class="mb-2 mt-2">
      <div class="relative flex cursor-pointer items-center pb-1 pt-1 transition-all duration-200 ease-in-out font-semibold text-green-700" style="padding-left: 0px;">
         <div class="absolute bottom-0 top-0 w-[2px] bg-green-600" style="left: 8px;"></div>
         <div class="flex flex-1 items-center gap-2 pl-4">
            <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="lucide lucide-chevron-up transition-transform rotate-180" aria-hidden="true">
               <path d="m18 15-6-6-6 6"></path>
            </svg>
            <div class="flex" data-state="closed"><a href="/leafwiki" data-discover="true"><span class="block max-w-[200px] overflow-hidden truncate text-ellipsis text-base font-semibold">Leafwiki</span></a></div>
         </div>
      </div>
      <div class="ml-4 pl-2 ">
         <div class="relative flex cursor-pointer items-center pb-1 pt-1 transition-all duration-200 ease-in-out text-gray-800 hover:bg-gray-100" style="padding-left: 16px;">
            <div class="absolute bottom-0 top-0 w-[2px] bg-gray-200" style="left: 8px;"></div>
            <div class="flex flex-1 items-center gap-2 pl-4">
               <div class="flex" data-state="closed"><a href="/leafwiki/vision" data-discover="true"><span class="block max-w-[200px] overflow-hidden truncate text-ellipsis text-sm text-gray-800">Vision</span></a></div>
            </div>
         </div>
         <div class="ml-4 pl-2 hidden"></div>
         <div class="relative flex cursor-pointer items-center pb-1 pt-1 transition-all duration-200 ease-in-out text-gray-800 hover:bg-gray-100" style="padding-left: 16px;">
            <div class="absolute bottom-0 top-0 w-[2px] bg-gray-200" style="left: 8px;"></div>
            <div class="flex flex-1 items-center gap-2 pl-4">
               <div class="flex" data-state="closed"><a href="/leafwiki/mission" data-discover="true"><span class="block max-w-[200px] overflow-hidden truncate text-ellipsis text-sm text-gray-800">Mission</span></a></div>
            </div>
         </div>
         <div class="ml-4 pl-2 hidden"></div>
         <div class="relative flex cursor-pointer items-center pb-1 pt-1 transition-all duration-200 ease-in-out text-gray-800 hover:bg-gray-100" style="padding-left: 16px;">
            <div class="absolute bottom-0 top-0 w-[2px] bg-gray-200" style="left: 8px;"></div>
            <div class="flex flex-1 items-center gap-2 pl-4">
               <div class="flex" data-state="closed"><a href="/leafwiki/dogfooding" data-discover="true"><span class="block max-w-[200px] overflow-hidden truncate text-ellipsis text-sm text-gray-800">Dogfooding</span></a></div>
            </div>
         </div>
         <div class="ml-4 pl-2 hidden"></div>
         <div class="relative flex cursor-pointer items-center pb-1 pt-1 transition-all duration-200 ease-in-out text-gray-800 hover:bg-gray-100" style="padding-left: 16px;">
            <div class="absolute bottom-0 top-0 w-[2px] bg-gray-200" style="left: 8px;"></div>
            <div class="flex flex-1 items-center gap-2 pl-4">
               <div class="flex" data-state="closed"><a href="/leafwiki/bugs-and-features" data-discover="true"><span class="block max-w-[200px] overflow-hidden truncate text-ellipsis text-sm text-gray-800">Bugs and Features</span></a></div>
            </div>
         </div>
         <div class="ml-4 pl-2 hidden"></div>
         <div class="relative flex cursor-pointer items-center pb-1 pt-1 transition-all duration-200 ease-in-out text-gray-800 hover:bg-gray-100" style="padding-left: 16px;">
            <div class="absolute bottom-0 top-0 w-[2px] bg-gray-200" style="left: 8px;"></div>
            <div class="flex flex-1 items-center gap-2 pl-4">
               <div class="flex" data-state="closed"><a href="/leafwiki/roadmap" data-discover="true"><span class="block max-w-[200px] overflow-hidden truncate text-ellipsis text-sm text-gray-800">Roadmap</span></a></div>
            </div>
         </div>
         <div class="ml-4 pl-2 hidden"></div>
         <div class="relative flex cursor-pointer items-center pb-1 pt-1 transition-all duration-200 ease-in-out text-gray-800 hover:bg-gray-100" style="padding-left: 16px;">
            <div class="absolute bottom-0 top-0 w-[2px] bg-gray-200" style="left: 8px;"></div>
            <div class="flex flex-1 items-center gap-2 pl-4">
               <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="lucide lucide-chevron-up transition-transform rotate-90" aria-hidden="true">
                  <path d="m18 15-6-6-6 6"></path>
               </svg>
               <div class="flex" data-state="closed"><a href="/leafwiki/projekttagebuch" data-discover="true"><span class="block max-w-[200px] overflow-hidden truncate text-ellipsis text-sm text-gray-800">Projekttagebuch</span></a></div>
            </div>
         </div>
         <div class="ml-4 pl-2 hidden">
            <div class="relative flex cursor-pointer items-center pb-1 pt-1 transition-all duration-200 ease-in-out text-gray-800 hover:bg-gray-100" style="padding-left: 32px;">
               <div class="absolute bottom-0 top-0 w-[2px] bg-gray-200" style="left: 8px;"></div>
               <div class="flex flex-1 items-center gap-2 pl-4">
                  <div class="flex" data-state="closed"><a href="/leafwiki/projekttagebuch/2025-04-16" data-discover="true"><span class="block max-w-[200px] overflow-hidden truncate text-ellipsis text-sm text-gray-500">2025-04-16</span></a></div>
               </div>
            </div>
            <div class="ml-4 pl-2 hidden"></div>
            <div class="relative flex cursor-pointer items-center pb-1 pt-1 transition-all duration-200 ease-in-out text-gray-800 hover:bg-gray-100" style="padding-left: 32px;">
               <div class="absolute bottom-0 top-0 w-[2px] bg-gray-200" style="left: 8px;"></div>
               <div class="flex flex-1 items-center gap-2 pl-4">
                  <div class="flex" data-state="closed"><a href="/leafwiki/projekttagebuch/2025-04-17" data-discover="true"><span class="block max-w-[200px] overflow-hidden truncate text-ellipsis text-sm text-gray-500">2025-04-17</span></a></div>
               </div>
            </div>
            <div class="ml-4 pl-2 hidden"></div>
            <div class="relative flex cursor-pointer items-center pb-1 pt-1 transition-all duration-200 ease-in-out text-gray-800 hover:bg-gray-100" style="padding-left: 32px;">
               <div class="absolute bottom-0 top-0 w-[2px] bg-gray-200" style="left: 8px;"></div>
               <div class="flex flex-1 items-center gap-2 pl-4">
                  <div class="flex" data-state="closed"><a href="/leafwiki/projekttagebuch/2025-04-18" data-discover="true"><span class="block max-w-[200px] overflow-hidden truncate text-ellipsis text-sm text-gray-500">2025-04-18</span></a></div>
               </div>
            </div>
            <div class="ml-4 pl-2 hidden"></div>
         </div>
         <div class="relative flex cursor-pointer items-center pb-1 pt-1 transition-all duration-200 ease-in-out text-gray-800 hover:bg-gray-100" style="padding-left: 16px;">
            <div class="absolute bottom-0 top-0 w-[2px] bg-gray-200" style="left: 8px;"></div>
            <div class="flex flex-1 items-center gap-2 pl-4">
               <div class="flex" data-state="closed"><a href="/leafwiki/markdown-testseite" data-discover="true"><span class="block max-w-[200px] overflow-hidden truncate text-ellipsis text-sm text-gray-800">Markdown Testseite</span></a></div>
            </div>
         </div>
         <div class="ml-4 pl-2 hidden"></div>
         <div class="relative flex cursor-pointer items-center pb-1 pt-1 transition-all duration-200 ease-in-out text-gray-800 hover:bg-gray-100" style="padding-left: 16px;">
            <div class="absolute bottom-0 top-0 w-[2px] bg-gray-200" style="left: 8px;"></div>
            <div class="flex flex-1 items-center gap-2 pl-4">
               <div class="flex" data-state="closed"><a href="/leafwiki/roadmap-post-launch" data-discover="true"><span class="block max-w-[200px] overflow-hidden truncate text-ellipsis text-sm text-gray-800">Roadmap - Post Launch</span></a></div>
            </div>
         </div>
         <div class="ml-4 pl-2 hidden"></div>
         <div class="relative flex cursor-pointer items-center pb-1 pt-1 transition-all duration-200 ease-in-out text-gray-800 hover:bg-gray-100" style="padding-left: 16px;">
            <div class="absolute bottom-0 top-0 w-[2px] bg-gray-200" style="left: 8px;"></div>
            <div class="flex flex-1 items-center gap-2 pl-4">
               <div class="flex" data-state="closed"><a href="/leafwiki/eine-neue-seite-mit-blatt" data-discover="true"><span class="block max-w-[200px] overflow-hidden truncate text-ellipsis text-sm text-gray-800">Eine neue Seite mit Blatt</span></a></div>
            </div>
         </div>
         <div class="ml-4 pl-2 hidden"></div>
         <div class="relative flex cursor-pointer items-center pb-1 pt-1 transition-all duration-200 ease-in-out text-gray-800 hover:bg-gray-100" style="padding-left: 16px;">
            <div class="absolute bottom-0 top-0 w-[2px] bg-gray-200" style="left: 8px;"></div>
            <div class="flex flex-1 items-center gap-2 pl-4">
               <div class="flex" data-state="closed"><a href="/leafwiki/code-mirror-test" data-discover="true"><span class="block max-w-[200px] overflow-hidden truncate text-ellipsis text-sm text-gray-800">Code Mirror Test</span></a></div>
            </div>
         </div>
         <div class="ml-4 pl-2 hidden"></div>
         <div class="relative flex cursor-pointer items-center pb-1 pt-1 transition-all duration-200 ease-in-out text-gray-800 hover:bg-gray-100" style="padding-left: 16px;">
            <div class="absolute bottom-0 top-0 w-[2px] bg-gray-200" style="left: 8px;"></div>
            <div class="flex flex-1 items-center gap-2 pl-4">
               <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="lucide lucide-chevron-up transition-transform rotate-90" aria-hidden="true">
                  <path d="m18 15-6-6-6 6"></path>
               </svg>
               <div class="flex" data-state="closed"><a href="/leafwiki/getting-started" data-discover="true"><span class="block max-w-[200px] overflow-hidden truncate text-ellipsis text-sm text-gray-800">Getting Started</span></a></div>
            </div>
         </div>
         <div class="ml-4 pl-2 hidden">
            <div class="relative flex cursor-pointer items-center pb-1 pt-1 transition-all duration-200 ease-in-out text-gray-800 hover:bg-gray-100" style="padding-left: 32px;">
               <div class="absolute bottom-0 top-0 w-[2px] bg-gray-200" style="left: 8px;"></div>
               <div class="flex flex-1 items-center gap-2 pl-4">
                  <div class="flex" data-state="closed"><a href="/leafwiki/getting-started/installation" data-discover="true"><span class="block max-w-[200px] overflow-hidden truncate text-ellipsis text-sm text-gray-500">Installation</span></a></div>
               </div>
            </div>
            <div class="ml-4 pl-2 hidden"></div>
         </div>
      </div>
      <div class="relative flex cursor-pointer items-center pb-1 pt-1 transition-all duration-200 ease-in-out text-gray-800 hover:bg-gray-100" style="padding-left: 0px;">
         <div class="absolute bottom-0 top-0 w-[2px] bg-gray-200" style="left: 8px;"></div>
         <div class="flex flex-1 items-center gap-2 pl-4">
            <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="lucide lucide-chevron-up transition-transform rotate-90" aria-hidden="true">
               <path d="m18 15-6-6-6 6"></path>
            </svg>
            <div class="flex" data-state="closed"><a href="/unwired-networks" data-discover="true"><span class="block max-w-[200px] overflow-hidden truncate text-ellipsis text-base font-semibold">Unwired Networks</span></a></div>
         </div>
      </div>
      <div class="ml-4 pl-2 hidden">
         <div class="relative flex cursor-pointer items-center pb-1 pt-1 transition-all duration-200 ease-in-out text-gray-800 hover:bg-gray-100" style="padding-left: 16px;">
            <div class="absolute bottom-0 top-0 w-[2px] bg-gray-200" style="left: 8px;"></div>
            <div class="flex flex-1 items-center gap-2 pl-4">
               <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="lucide lucide-chevron-up transition-transform rotate-90" aria-hidden="true">
                  <path d="m18 15-6-6-6 6"></path>
               </svg>
               <div class="flex" data-state="closed"><a href="/unwired-networks/todos" data-discover="true"><span class="block max-w-[200px] overflow-hidden truncate text-ellipsis text-sm text-gray-800">Todos</span></a></div>
            </div>
         </div>
         <div class="ml-4 pl-2 hidden">
            <div class="relative flex cursor-pointer items-center pb-1 pt-1 transition-all duration-200 ease-in-out text-gray-800 hover:bg-gray-100" style="padding-left: 32px;">
               <div class="absolute bottom-0 top-0 w-[2px] bg-gray-200" style="left: 8px;"></div>
               <div class="flex flex-1 items-center gap-2 pl-4">
                  <div class="flex" data-state="closed"><a href="/unwired-networks/todos/2025-04-09-todos" data-discover="true"><span class="block max-w-[200px] overflow-hidden truncate text-ellipsis text-sm text-gray-500">2025-04-09 Todos</span></a></div>
               </div>
            </div>
            <div class="ml-4 pl-2 hidden"></div>
            <div class="relative flex cursor-pointer items-center pb-1 pt-1 transition-all duration-200 ease-in-out text-gray-800 hover:bg-gray-100" style="padding-left: 32px;">
               <div class="absolute bottom-0 top-0 w-[2px] bg-gray-200" style="left: 8px;"></div>
               <div class="flex flex-1 items-center gap-2 pl-4">
                  <div class="flex" data-state="closed"><a href="/unwired-networks/todos/2025-04-08-todos" data-discover="true"><span class="block max-w-[200px] overflow-hidden truncate text-ellipsis text-sm text-gray-500">2025-04-08 Todos</span></a></div>
               </div>
            </div>
            <div class="ml-4 pl-2 hidden"></div>
            <div class="relative flex cursor-pointer items-center pb-1 pt-1 transition-all duration-200 ease-in-out text-gray-800 hover:bg-gray-100" style="padding-left: 32px;">
               <div class="absolute bottom-0 top-0 w-[2px] bg-gray-200" style="left: 8px;"></div>
               <div class="flex flex-1 items-center gap-2 pl-4">
                  <div class="flex" data-state="closed"><a href="/unwired-networks/todos/2025-04-07-todos" data-discover="true"><span class="block max-w-[200px] overflow-hidden truncate text-ellipsis text-sm text-gray-500">2025-04-07 Todos</span></a></div>
               </div>
            </div>
            <div class="ml-4 pl-2 hidden"></div>
         </div>
         <div class="relative flex cursor-pointer items-center pb-1 pt-1 transition-all duration-200 ease-in-out text-gray-800 hover:bg-gray-100" style="padding-left: 16px;">
            <div class="absolute bottom-0 top-0 w-[2px] bg-gray-200" style="left: 8px;"></div>
            <div class="flex flex-1 items-center gap-2 pl-4">
               <div class="flex" data-state="closed"><a href="/unwired-networks/github-token" data-discover="true"><span class="block max-w-[200px] overflow-hidden truncate text-ellipsis text-sm text-gray-800">GitHub Token</span></a></div>
            </div>
         </div>
         <div class="ml-4 pl-2 hidden"></div>
         <div class="relative flex cursor-pointer items-center pb-1 pt-1 transition-all duration-200 ease-in-out text-gray-800 hover:bg-gray-100" style="padding-left: 16px;">
            <div class="absolute bottom-0 top-0 w-[2px] bg-gray-200" style="left: 8px;"></div>
            <div class="flex flex-1 items-center gap-2 pl-4">
               <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="lucide lucide-chevron-up transition-transform rotate-90" aria-hidden="true">
                  <path d="m18 15-6-6-6 6"></path>
               </svg>
               <div class="flex" data-state="closed"><a href="/unwired-networks/migration-jenkins" data-discover="true"><span class="block max-w-[200px] overflow-hidden truncate text-ellipsis text-sm text-gray-800">Migration Jenkins</span></a></div>
            </div>
         </div>
         <div class="ml-4 pl-2 hidden">
            <div class="relative flex cursor-pointer items-center pb-1 pt-1 transition-all duration-200 ease-in-out text-gray-800 hover:bg-gray-100" style="padding-left: 32px;">
               <div class="absolute bottom-0 top-0 w-[2px] bg-gray-200" style="left: 8px;"></div>
               <div class="flex flex-1 items-center gap-2 pl-4">
                  <div class="flex" data-state="closed"><a href="/unwired-networks/migration-jenkins/office-staging" data-discover="true"><span class="block max-w-[200px] overflow-hidden truncate text-ellipsis text-sm text-gray-500">Office Staging Stargate Migration</span></a></div>
               </div>
            </div>
            <div class="ml-4 pl-2 hidden"></div>
         </div>
         <div class="relative flex cursor-pointer items-center pb-1 pt-1 transition-all duration-200 ease-in-out text-gray-800 hover:bg-gray-100" style="padding-left: 16px;">
            <div class="absolute bottom-0 top-0 w-[2px] bg-gray-200" style="left: 8px;"></div>
            <div class="flex flex-1 items-center gap-2 pl-4">
               <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="lucide lucide-chevron-up transition-transform rotate-90" aria-hidden="true">
                  <path d="m18 15-6-6-6 6"></path>
               </svg>
               <div class="flex" data-state="closed"><a href="/unwired-networks/jenkins-migration" data-discover="true"><span class="block max-w-[200px] overflow-hidden truncate text-ellipsis text-sm text-gray-800">Jenkins Migration</span></a></div>
            </div>
         </div>
         <div class="ml-4 pl-2 hidden">
            <div class="relative flex cursor-pointer items-center pb-1 pt-1 transition-all duration-200 ease-in-out text-gray-800 hover:bg-gray-100" style="padding-left: 32px;">
               <div class="absolute bottom-0 top-0 w-[2px] bg-gray-200" style="left: 8px;"></div>
               <div class="flex flex-1 items-center gap-2 pl-4">
                  <div class="flex" data-state="closed"><a href="/unwired-networks/jenkins-migration/cicd-config-as-code" data-discover="true"><span class="block max-w-[200px] overflow-hidden truncate text-ellipsis text-sm text-gray-500">CICD Config as Code</span></a></div>
               </div>
            </div>
            <div class="ml-4 pl-2 hidden"></div>
            <div class="relative flex cursor-pointer items-center pb-1 pt-1 transition-all duration-200 ease-in-out text-gray-800 hover:bg-gray-100" style="padding-left: 32px;">
               <div class="absolute bottom-0 top-0 w-[2px] bg-gray-200" style="left: 8px;"></div>
               <div class="flex flex-1 items-center gap-2 pl-4">
                  <div class="flex" data-state="closed"><a href="/unwired-networks/jenkins-migration/internal-config-as-code" data-discover="true"><span class="block max-w-[200px] overflow-hidden truncate text-ellipsis text-sm text-gray-500">Internal Config as Code</span></a></div>
               </div>
            </div>
            <div class="ml-4 pl-2 hidden"></div>
            <div class="relative flex cursor-pointer items-center pb-1 pt-1 transition-all duration-200 ease-in-out text-gray-800 hover:bg-gray-100" style="padding-left: 32px;">
               <div class="absolute bottom-0 top-0 w-[2px] bg-gray-200" style="left: 8px;"></div>
               <div class="flex flex-1 items-center gap-2 pl-4">
                  <div class="flex" data-state="closed"><a href="/unwired-networks/jenkins-migration/secrets" data-discover="true"><span class="block max-w-[200px] overflow-hidden truncate text-ellipsis text-sm text-gray-500">Secrets</span></a></div>
               </div>
            </div>
            <div class="ml-4 pl-2 hidden"></div>
            <div class="relative flex cursor-pointer items-center pb-1 pt-1 transition-all duration-200 ease-in-out text-gray-800 hover:bg-gray-100" style="padding-left: 32px;">
               <div class="absolute bottom-0 top-0 w-[2px] bg-gray-200" style="left: 8px;"></div>
               <div class="flex flex-1 items-center gap-2 pl-4">
                  <div class="flex" data-state="closed"><a href="/unwired-networks/jenkins-migration/jenkins-migration-plan" data-discover="true"><span class="block max-w-[200px] overflow-hidden truncate text-ellipsis text-sm text-gray-500">Jenkins Migration Plan</span></a></div>
               </div>
            </div>
            <div class="ml-4 pl-2 hidden"></div>
            <div class="relative flex cursor-pointer items-center pb-1 pt-1 transition-all duration-200 ease-in-out text-gray-800 hover:bg-gray-100" style="padding-left: 32px;">
               <div class="absolute bottom-0 top-0 w-[2px] bg-gray-200" style="left: 8px;"></div>
               <div class="flex flex-1 items-center gap-2 pl-4">
                  <div class="flex" data-state="closed"><a href="/unwired-networks/jenkins-migration/jenkins-slave-build-config" data-discover="true"><span class="block max-w-[200px] overflow-hidden truncate text-ellipsis text-sm text-gray-500">Jenkins-Slave Build Config</span></a></div>
               </div>
            </div>
            <div class="ml-4 pl-2 hidden"></div>
            <div class="relative flex cursor-pointer items-center pb-1 pt-1 transition-all duration-200 ease-in-out text-gray-800 hover:bg-gray-100" style="padding-left: 32px;">
               <div class="absolute bottom-0 top-0 w-[2px] bg-gray-200" style="left: 8px;"></div>
               <div class="flex flex-1 items-center gap-2 pl-4">
                  <div class="flex" data-state="closed"><a href="/unwired-networks/jenkins-migration/subpage-under-subpage" data-discover="true"><span class="block max-w-[200px] overflow-hidden truncate text-ellipsis text-sm text-gray-500">subpage under subpage</span></a></div>
               </div>
            </div>
            <div class="ml-4 pl-2 hidden"></div>
            <div class="relative flex cursor-pointer items-center pb-1 pt-1 transition-all duration-200 ease-in-out text-gray-800 hover:bg-gray-100" style="padding-left: 32px;">
               <div class="absolute bottom-0 top-0 w-[2px] bg-gray-200" style="left: 8px;"></div>
               <div class="flex flex-1 items-center gap-2 pl-4">
                  <div class="flex" data-state="closed"><a href="/unwired-networks/jenkins-migration/one-more" data-discover="true"><span class="block max-w-[200px] overflow-hidden truncate text-ellipsis text-sm text-gray-500">One more</span></a></div>
               </div>
            </div>
            <div class="ml-4 pl-2 hidden"></div>
         </div>
         <div class="relative flex cursor-pointer items-center pb-1 pt-1 transition-all duration-200 ease-in-out text-gray-800 hover:bg-gray-100" style="padding-left: 16px;">
            <div class="absolute bottom-0 top-0 w-[2px] bg-gray-200" style="left: 8px;"></div>
            <div class="flex flex-1 items-center gap-2 pl-4">
               <div class="flex" data-state="closed"><a href="/unwired-networks/ci-cd-migration" data-discover="true"><span class="block max-w-[200px] overflow-hidden truncate text-ellipsis text-sm text-gray-800">CI/CD-Migration</span></a></div>
            </div>
         </div>
         <div class="ml-4 pl-2 hidden"></div>
         <div class="relative flex cursor-pointer items-center pb-1 pt-1 transition-all duration-200 ease-in-out text-gray-800 hover:bg-gray-100" style="padding-left: 16px;">
            <div class="absolute bottom-0 top-0 w-[2px] bg-gray-200" style="left: 8px;"></div>
            <div class="flex flex-1 items-center gap-2 pl-4">
               <div class="flex" data-state="closed"><a href="/unwired-networks/ci-cd-cluster-architecture" data-discover="true"><span class="block max-w-[200px] overflow-hidden truncate text-ellipsis text-sm text-gray-800">CI/CD Cluster Architecture</span></a></div>
            </div>
         </div>
         <div class="ml-4 pl-2 hidden"></div>
      </div>
      <div class="relative flex cursor-pointer items-center pb-1 pt-1 transition-all duration-200 ease-in-out text-gray-800 hover:bg-gray-100" style="padding-left: 0px;">
         <div class="absolute bottom-0 top-0 w-[2px] bg-gray-200" style="left: 8px;"></div>
         <div class="flex flex-1 items-center gap-2 pl-4">
            <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="lucide lucide-chevron-up transition-transform rotate-90" aria-hidden="true">
               <path d="m18 15-6-6-6 6"></path>
            </svg>
            <div class="flex" data-state="closed"><a href="/socradev" data-discover="true"><span class="block max-w-[200px] overflow-hidden truncate text-ellipsis text-base font-semibold">Socradev</span></a></div>
         </div>
      </div>
      <div class="ml-4 pl-2 hidden">
         <div class="relative flex cursor-pointer items-center pb-1 pt-1 transition-all duration-200 ease-in-out text-gray-800 hover:bg-gray-100" style="padding-left: 16px;">
            <div class="absolute bottom-0 top-0 w-[2px] bg-gray-200" style="left: 8px;"></div>
            <div class="flex flex-1 items-center gap-2 pl-4">
               <div class="flex" data-state="closed"><a href="/socradev/plane" data-discover="true"><span class="block max-w-[200px] overflow-hidden truncate text-ellipsis text-sm text-gray-800">Pläne</span></a></div>
            </div>
         </div>
         <div class="ml-4 pl-2 hidden"></div>
         <div class="relative flex cursor-pointer items-center pb-1 pt-1 transition-all duration-200 ease-in-out text-gray-800 hover:bg-gray-100" style="padding-left: 16px;">
            <div class="absolute bottom-0 top-0 w-[2px] bg-gray-200" style="left: 8px;"></div>
            <div class="flex flex-1 items-center gap-2 pl-4">
               <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="lucide lucide-chevron-up transition-transform rotate-90" aria-hidden="true">
                  <path d="m18 15-6-6-6 6"></path>
               </svg>
               <div class="flex" data-state="closed"><a href="/socradev/risiken" data-discover="true"><span class="block max-w-[200px] overflow-hidden truncate text-ellipsis text-sm text-gray-800">Risiken</span></a></div>
            </div>
         </div>
         <div class="ml-4 pl-2 hidden">
            <div class="relative flex cursor-pointer items-center pb-1 pt-1 transition-all duration-200 ease-in-out text-gray-800 hover:bg-gray-100" style="padding-left: 32px;">
               <div class="absolute bottom-0 top-0 w-[2px] bg-gray-200" style="left: 8px;"></div>
               <div class="flex flex-1 items-center gap-2 pl-4">
                  <div class="flex" data-state="closed"><a href="/socradev/risiken/risiken-und-nebenwrikungen" data-discover="true"><span class="block max-w-[200px] overflow-hidden truncate text-ellipsis text-sm text-gray-500">Risiken und Nebenwrikungen</span></a></div>
               </div>
            </div>
            <div class="ml-4 pl-2 hidden"></div>
         </div>
         <div class="relative flex cursor-pointer items-center pb-1 pt-1 transition-all duration-200 ease-in-out text-gray-800 hover:bg-gray-100" style="padding-left: 16px;">
            <div class="absolute bottom-0 top-0 w-[2px] bg-gray-200" style="left: 8px;"></div>
            <div class="flex flex-1 items-center gap-2 pl-4">
               <div class="flex" data-state="closed"><a href="/socradev/argocd" data-discover="true"><span class="block max-w-[200px] overflow-hidden truncate text-ellipsis text-sm text-gray-800">ArgoCD</span></a></div>
            </div>
         </div>
         <div class="ml-4 pl-2 hidden"></div>
         <div class="relative flex cursor-pointer items-center pb-1 pt-1 transition-all duration-200 ease-in-out text-gray-800 hover:bg-gray-100" style="padding-left: 16px;">
            <div class="absolute bottom-0 top-0 w-[2px] bg-gray-200" style="left: 8px;"></div>
            <div class="flex flex-1 items-center gap-2 pl-4">
               <div class="flex" data-state="closed"><a href="/socradev/kubernetes-distributionen" data-discover="true"><span class="block max-w-[200px] overflow-hidden truncate text-ellipsis text-sm text-gray-800">Kubernetes Distributionen</span></a></div>
            </div>
         </div>
         <div class="ml-4 pl-2 hidden"></div>
         <div class="relative flex cursor-pointer items-center pb-1 pt-1 transition-all duration-200 ease-in-out text-gray-800 hover:bg-gray-100" style="padding-left: 16px;">
            <div class="absolute bottom-0 top-0 w-[2px] bg-gray-200" style="left: 8px;"></div>
            <div class="flex flex-1 items-center gap-2 pl-4">
               <div class="flex" data-state="closed"><a href="/socradev/prometheus" data-discover="true"><span class="block max-w-[200px] overflow-hidden truncate text-ellipsis text-sm text-gray-800">Prometheus</span></a></div>
            </div>
         </div>
         <div class="ml-4 pl-2 hidden"></div>
         <div class="relative flex cursor-pointer items-center pb-1 pt-1 transition-all duration-200 ease-in-out text-gray-800 hover:bg-gray-100" style="padding-left: 16px;">
            <div class="absolute bottom-0 top-0 w-[2px] bg-gray-200" style="left: 8px;"></div>
            <div class="flex flex-1 items-center gap-2 pl-4">
               <div class="flex" data-state="closed"><a href="/socradev/fluxcd" data-discover="true"><span class="block max-w-[200px] overflow-hidden truncate text-ellipsis text-sm text-gray-800">FluxCD</span></a></div>
            </div>
         </div>
         <div class="ml-4 pl-2 hidden"></div>
         <div class="relative flex cursor-pointer items-center pb-1 pt-1 transition-all duration-200 ease-in-out text-gray-800 hover:bg-gray-100" style="padding-left: 16px;">
            <div class="absolute bottom-0 top-0 w-[2px] bg-gray-200" style="left: 8px;"></div>
            <div class="flex flex-1 items-center gap-2 pl-4">
               <div class="flex" data-state="closed"><a href="/socradev/calico" data-discover="true"><span class="block max-w-[200px] overflow-hidden truncate text-ellipsis text-sm text-gray-800">Calico</span></a></div>
            </div>
         </div>
         <div class="ml-4 pl-2 hidden"></div>
         <div class="relative flex cursor-pointer items-center pb-1 pt-1 transition-all duration-200 ease-in-out text-gray-800 hover:bg-gray-100" style="padding-left: 16px;">
            <div class="absolute bottom-0 top-0 w-[2px] bg-gray-200" style="left: 8px;"></div>
            <div class="flex flex-1 items-center gap-2 pl-4">
               <div class="flex" data-state="closed"><a href="/socradev/zertifizie" data-discover="true"><span class="block max-w-[200px] overflow-hidden truncate text-ellipsis text-sm text-gray-800">Zertifizierungen</span></a></div>
            </div>
         </div>
         <div class="ml-4 pl-2 hidden"></div>
         <div class="relative flex cursor-pointer items-center pb-1 pt-1 transition-all duration-200 ease-in-out text-gray-800 hover:bg-gray-100" style="padding-left: 16px;">
            <div class="absolute bottom-0 top-0 w-[2px] bg-gray-200" style="left: 8px;"></div>
            <div class="flex flex-1 items-center gap-2 pl-4">
               <div class="flex" data-state="closed"><a href="/socradev/mitarbeiter" data-discover="true"><span class="block max-w-[200px] overflow-hidden truncate text-ellipsis text-sm text-gray-800">Mitarbeiter</span></a></div>
            </div>
         </div>
         <div class="ml-4 pl-2 hidden"></div>
         <div class="relative flex cursor-pointer items-center pb-1 pt-1 transition-all duration-200 ease-in-out text-gray-800 hover:bg-gray-100" style="padding-left: 16px;">
            <div class="absolute bottom-0 top-0 w-[2px] bg-gray-200" style="left: 8px;"></div>
            <div class="flex flex-1 items-center gap-2 pl-4">
               <div class="flex" data-state="closed"><a href="/socradev/datacorp-kundentermin" data-discover="true"><span class="block max-w-[200px] overflow-hidden truncate text-ellipsis text-sm text-gray-800">DataCorp Kundentermin</span></a></div>
            </div>
         </div>
         <div class="ml-4 pl-2 hidden"></div>
      </div>
      <div class="relative flex cursor-pointer items-center pb-1 pt-1 transition-all duration-200 ease-in-out text-gray-800 hover:bg-gray-100" style="padding-left: 0px;">
         <div class="absolute bottom-0 top-0 w-[2px] bg-gray-200" style="left: 8px;"></div>
         <div class="flex flex-1 items-center gap-2 pl-4">
            <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="lucide lucide-chevron-up transition-transform rotate-90" aria-hidden="true">
               <path d="m18 15-6-6-6 6"></path>
            </svg>
            <div class="flex" data-state="closed"><a href="/nice-to-know" data-discover="true"><span class="block max-w-[200px] overflow-hidden truncate text-ellipsis text-base font-semibold">Nice to Know</span></a></div>
         </div>
      </div>
      <div class="ml-4 pl-2 hidden">
         <div class="relative flex cursor-pointer items-center pb-1 pt-1 transition-all duration-200 ease-in-out text-gray-800 hover:bg-gray-100" style="padding-left: 16px;">
            <div class="absolute bottom-0 top-0 w-[2px] bg-gray-200" style="left: 8px;"></div>
            <div class="flex flex-1 items-center gap-2 pl-4">
               <div class="flex" data-state="closed"><a href="/nice-to-know/jenkins" data-discover="true"><span class="block max-w-[200px] overflow-hidden truncate text-ellipsis text-sm text-gray-800">Jenkins</span></a></div>
            </div>
         </div>
         <div class="ml-4 pl-2 hidden"></div>
         <div class="relative flex cursor-pointer items-center pb-1 pt-1 transition-all duration-200 ease-in-out text-gray-800 hover:bg-gray-100" style="padding-left: 16px;">
            <div class="absolute bottom-0 top-0 w-[2px] bg-gray-200" style="left: 8px;"></div>
            <div class="flex flex-1 items-center gap-2 pl-4">
               <div class="flex" data-state="closed"><a href="/nice-to-know/service-account-jenkins" data-discover="true"><span class="block max-w-[200px] overflow-hidden truncate text-ellipsis text-sm text-gray-800">Service Account Jenkins</span></a></div>
            </div>
         </div>
         <div class="ml-4 pl-2 hidden"></div>
         <div class="relative flex cursor-pointer items-center pb-1 pt-1 transition-all duration-200 ease-in-out text-gray-800 hover:bg-gray-100" style="padding-left: 16px;">
            <div class="absolute bottom-0 top-0 w-[2px] bg-gray-200" style="left: 8px;"></div>
            <div class="flex flex-1 items-center gap-2 pl-4">
               <div class="flex" data-state="closed"><a href="/nice-to-know/test-kubeconfig" data-discover="true"><span class="block max-w-[200px] overflow-hidden truncate text-ellipsis text-sm text-gray-800">Test kubeconfig</span></a></div>
            </div>
         </div>
         <div class="ml-4 pl-2 hidden"></div>
      </div>
      <div class="relative flex cursor-pointer items-center pb-1 pt-1 transition-all duration-200 ease-in-out text-gray-800 hover:bg-gray-100" style="padding-left: 0px;">
         <div class="absolute bottom-0 top-0 w-[2px] bg-gray-200" style="left: 8px;"></div>
         <div class="flex flex-1 items-center gap-2 pl-4">
            <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="lucide lucide-chevron-up transition-transform rotate-90" aria-hidden="true">
               <path d="m18 15-6-6-6 6"></path>
            </svg>
            <div class="flex" data-state="closed"><a href="/privates" data-discover="true"><span class="block max-w-[200px] overflow-hidden truncate text-ellipsis text-base font-semibold">Privates</span></a></div>
         </div>
      </div>
      <div class="ml-4 pl-2 hidden">
         <div class="relative flex cursor-pointer items-center pb-1 pt-1 transition-all duration-200 ease-in-out text-gray-800 hover:bg-gray-100" style="padding-left: 16px;">
            <div class="absolute bottom-0 top-0 w-[2px] bg-gray-200" style="left: 8px;"></div>
            <div class="flex flex-1 items-center gap-2 pl-4">
               <div class="flex" data-state="closed"><a href="/privates/autounfall" data-discover="true"><span class="block max-w-[200px] overflow-hidden truncate text-ellipsis text-sm text-gray-800">Autounfall</span></a></div>
            </div>
         </div>
         <div class="ml-4 pl-2 hidden"></div>
         <div class="relative flex cursor-pointer items-center pb-1 pt-1 transition-all duration-200 ease-in-out text-gray-800 hover:bg-gray-100" style="padding-left: 16px;">
            <div class="absolute bottom-0 top-0 w-[2px] bg-gray-200" style="left: 8px;"></div>
            <div class="flex flex-1 items-center gap-2 pl-4">
               <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="lucide lucide-chevron-up transition-transform rotate-90" aria-hidden="true">
                  <path d="m18 15-6-6-6 6"></path>
               </svg>
               <div class="flex" data-state="closed"><a href="/privates/padel-tennis" data-discover="true"><span class="block max-w-[200px] overflow-hidden truncate text-ellipsis text-sm text-gray-800">Padel tennis</span></a></div>
            </div>
         </div>
         <div class="ml-4 pl-2 hidden">
            <div class="relative flex cursor-pointer items-center pb-1 pt-1 transition-all duration-200 ease-in-out text-gray-800 hover:bg-gray-100" style="padding-left: 32px;">
               <div class="absolute bottom-0 top-0 w-[2px] bg-gray-200" style="left: 8px;"></div>
               <div class="flex flex-1 items-center gap-2 pl-4">
                  <div class="flex" data-state="closed"><a href="/privates/padel-tennis/tagebuch" data-discover="true"><span class="block max-w-[200px] overflow-hidden truncate text-ellipsis text-sm text-gray-500">Tagebuch</span></a></div>
               </div>
            </div>
            <div class="ml-4 pl-2 hidden"></div>
         </div>
      </div>
      <div class="relative flex cursor-pointer items-center pb-1 pt-1 transition-all duration-200 ease-in-out text-gray-800 hover:bg-gray-100" style="padding-left: 0px;">
         <div class="absolute bottom-0 top-0 w-[2px] bg-gray-200" style="left: 8px;"></div>
         <div class="flex flex-1 items-center gap-2 pl-4">
            <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="lucide lucide-chevron-up transition-transform rotate-90" aria-hidden="true">
               <path d="m18 15-6-6-6 6"></path>
            </svg>
            <div class="flex" data-state="closed"><a href="/devops" data-discover="true"><span class="block max-w-[200px] overflow-hidden truncate text-ellipsis text-base font-semibold">DevOps</span></a></div>
         </div>
      </div>
      <div class="ml-4 pl-2 hidden">
         <div class="relative flex cursor-pointer items-center pb-1 pt-1 transition-all duration-200 ease-in-out text-gray-800 hover:bg-gray-100" style="padding-left: 16px;">
            <div class="absolute bottom-0 top-0 w-[2px] bg-gray-200" style="left: 8px;"></div>
            <div class="flex flex-1 items-center gap-2 pl-4">
               <div class="flex" data-state="closed"><a href="/devops/architecture" data-discover="true"><span class="block max-w-[200px] overflow-hidden truncate text-ellipsis text-sm text-gray-800">Architecture</span></a></div>
            </div>
         </div>
         <div class="ml-4 pl-2 hidden"></div>
         <div class="relative flex cursor-pointer items-center pb-1 pt-1 transition-all duration-200 ease-in-out text-gray-800 hover:bg-gray-100" style="padding-left: 16px;">
            <div class="absolute bottom-0 top-0 w-[2px] bg-gray-200" style="left: 8px;"></div>
            <div class="flex flex-1 items-center gap-2 pl-4">
               <div class="flex" data-state="closed"><a href="/devops/implemente" data-discover="true"><span class="block max-w-[200px] overflow-hidden truncate text-ellipsis text-sm text-gray-800">Implementation</span></a></div>
            </div>
         </div>
         <div class="ml-4 pl-2 hidden"></div>
         <div class="relative flex cursor-pointer items-center pb-1 pt-1 transition-all duration-200 ease-in-out text-gray-800 hover:bg-gray-100" style="padding-left: 16px;">
            <div class="absolute bottom-0 top-0 w-[2px] bg-gray-200" style="left: 8px;"></div>
            <div class="flex flex-1 items-center gap-2 pl-4">
               <div class="flex" data-state="closed"><a href="/devops/processes" data-discover="true"><span class="block max-w-[200px] overflow-hidden truncate text-ellipsis text-sm text-gray-800">Processes</span></a></div>
            </div>
         </div>
         <div class="ml-4 pl-2 hidden"></div>
      </div>
   </div>
</div>
**/

func (r *NavigationRenderer) Render(items []NavigationItem) string {
	var htmlBuilder strings.Builder
	htmlBuilder.WriteString("<div class=\"flex flex-1 flex-col\">\n")
	htmlBuilder.WriteString(r.RenderItem(items, 0))
	htmlBuilder.WriteString("</div>\n")
	return htmlBuilder.String()
}

func (r *NavigationRenderer) RenderItem(items []NavigationItem, depth int) string {
	var htmlBuilder strings.Builder
	for _, item := range items {
		paddingLeft := depth * 16 // 16px padding for each depth level

		fontStyle := "text-sm"
		fontColor := "text-gray-500"

		if depth == 0 {
			fontStyle = "text-base font-semibold"
			fontColor = ""
		}

		if depth == 1 {
			fontStyle = "text-sm"
			fontColor = "text-gray-800"
		}

		htmlBuilder.WriteString(fmt.Sprintf("<div class=\"relative flex cursor-pointer items-center pb-1 pt-1 transition-all duration-200 ease-in-out text-gray-800 hover:bg-gray-100\" style=\"padding-left: %dpx;\">\n", paddingLeft))
		htmlBuilder.WriteString("<div class=\"absolute bottom-0 top-0 w-[2px] bg-gray-200\" style=\"left: 8px;\"></div>\n")
		htmlBuilder.WriteString("<div class=\"flex flex-1 items-center gap-2 pl-4\">\n")
		if len(item.Children) > 0 {
			htmlBuilder.WriteString("<svg xmlns=\"http://www.w3.org/2000/svg\" width=\"16\" height=\"16\" viewBox=\"0 0 24 24\" fill=\"none\" stroke=\"currentColor\" stroke-width=\"2\" stroke-linecap=\"round\" stroke-linejoin=\"round\" class=\"lucide lucide-chevron-up transition-transform rotate-90\" aria-hidden=\"true\">\n")
			htmlBuilder.WriteString("<path d=\"m18 15-6-6-6 6\"></path>\n")
			htmlBuilder.WriteString("</svg>\n")
		}
		htmlBuilder.WriteString(fmt.Sprintf("<div class=\"flex\"><a href=\"%s\" data-discover=\"true\"><span class=\"block max-w-[200px] overflow-hidden truncate text-ellipsis %s %s\">%s</span></a></div>\n", fontColor, fontStyle, item.URL, item.Title))
		htmlBuilder.WriteString("</div>\n")
		if len(item.Children) > 0 {
			htmlBuilder.WriteString("<div class=\"ml-4 pl-2 hidden\">\n")
			htmlBuilder.WriteString(r.RenderItem(item.Children, depth+1))
			htmlBuilder.WriteString("</div>\n")
		}
		htmlBuilder.WriteString("</div>\n")
	}
	return htmlBuilder.String()
}
