import { isTraditionalUiLanguage } from './language.js'

export function isTraditionalProjectLanguage(language) {
  return isTraditionalUiLanguage(language)
}

export function localizedProjectName(project, language) {
  if (!project) return ''
  if (isTraditionalProjectLanguage(language)) {
    return project.nameCn || project.name || project.nameEn || ''
  }
  return project.nameEn || project.name || project.nameCn || ''
}
